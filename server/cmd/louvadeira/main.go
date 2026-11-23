package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"louvadeira/internal/clock"
	"louvadeira/internal/config"
	"louvadeira/internal/httpserver"
	"louvadeira/internal/messenger"
	"louvadeira/internal/notify"
	"louvadeira/internal/storage"
	"louvadeira/internal/store"
	"louvadeira/internal/webui"
	"louvadeira/internal/whatsapp"
)

const usage = `uso:
  louvadeira serve                          inicia o servidor (padrão)
  louvadeira admin add -name N -phone T     cadastra (ou promove) um administrador
  louvadeira admin link -phone T            gera um link de acesso de 15 minutos
  louvadeira whatsapp link [-phone T]       vincula um número (QR Code, ou código com -phone)
  louvadeira whatsapp logout                desvincula o número`

const loginLinkTTL = 15 * time.Minute

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if len(args) == 0 || args[0] == "serve" {
		return serve(ctx, cfg, log)
	}
	if args[0] == "whatsapp" && len(args) > 1 {
		return whatsappCommand(ctx, cfg, log, args[1], args[2:])
	}
	if args[0] == "admin" && len(args) > 1 {
		return adminCommand(ctx, cfg, args[1], args[2:])
	}
	return errors.New(usage)
}

func serve(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	db, err := storage.Open(ctx, cfg.AppDBPath())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		return err
	}

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return fmt.Errorf("fuso horário %q: %w", cfg.Timezone, err)
	}
	registry := store.New(db)
	if n, err := registry.DropUnsentClaims(ctx); err != nil {
		return err
	} else if n > 0 {
		log.Warn("mensagens interrompidas na última parada; vão sair de novo", "quantas", n)
	}
	bot := &notify.Bot{Store: registry, Log: log, Location: location, Settle: 20 * time.Second, ClockSynced: clock.Synced}
	deps := httpserver.Deps{
		Log:          log,
		UI:           webui.FS(),
		Store:        registry,
		Bot:          bot,
		WhatsAppMode: string(cfg.WhatsAppMode),
		AdminToken:   cfg.AdminToken,
		PublicURL:    cfg.PublicURL,
	}
	if cfg.WhatsAppMode != config.WhatsAppOff {
		wa, err := whatsapp.Open(ctx, cfg.WhatsAppDBPath(), log)
		if err != nil {
			return err
		}
		defer wa.Close()
		deps.WhatsApp = wa
		deps.Messenger = messenger.Paced(wa, 2*time.Second, 5*time.Second)
		if cfg.WhatsAppMode == config.WhatsAppTest {
			deps.Messenger = messenger.WithAllowlist(deps.Messenger, addresses(cfg.WhatsAppAllowlist))
		}
		bot.Messenger = deps.Messenger
		bot.Async = func(task func()) { go task() }
		bot.OwnPhone = wa.OwnPhone
		wa.OnVote(bot.HandleVote)
		wa.OnText(bot.HandleText)
		go keepConnecting(ctx, wa, log)
		scheduler := make(chan struct{})
		go func() {
			defer close(scheduler)
			runScheduler(ctx, bot, wa)
		}()
		// NOTE: Waits for the message in flight, so a stop never cuts a send in half.
		defer func() {
			cancel()
			<-scheduler
		}()
	}

	server := &http.Server{Addr: cfg.Addr, Handler: httpserver.New(deps), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	log.Info("servidor no ar", "endereço", cfg.Addr, "whatsapp", cfg.WhatsAppMode)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// NOTE: whatsmeow reconnects on its own once connected; this loop only covers a first connect that fails.
func keepConnecting(ctx context.Context, wa *whatsapp.Client, log *slog.Logger) {
	for {
		if !wa.Status().Linked {
			log.Warn("WhatsApp não vinculado; rode 'louvadeira whatsapp link' e reinicie o serviço")
			return
		}
		err := wa.Connect(ctx)
		if err == nil {
			return
		}
		log.Error("falha ao conectar no WhatsApp; nova tentativa em 30s", "erro", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}
}

/**
 * NOTE: Checks for due messages every five minutes while WhatsApp is connected; the bot itself keeps quiet hours and
 * remembers what it already sent.
 */
func runScheduler(ctx context.Context, bot *notify.Bot, wa *whatsapp.Client) {
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	replayed := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if wa.Status().Connected {
				if !replayed {
					bot.ReplayPendingVotes(ctx)
					replayed = true
				}
				bot.Tick(ctx)
			}
			timer.Reset(5 * time.Minute)
		}
	}
}

func whatsappCommand(ctx context.Context, cfg config.Config, log *slog.Logger, command string, args []string) error {
	wa, err := whatsapp.Open(ctx, cfg.WhatsAppDBPath(), log)
	if err != nil {
		return err
	}
	defer wa.Close()

	switch command {
	case "link":
		flags := flag.NewFlagSet("link", flag.ContinueOnError)
		phone := flags.String("phone", "", "número com código do país, para parear por código em vez de QR Code")
		if err := flags.Parse(args); err != nil {
			return err
		}
		if err := wa.Link(ctx, *phone, os.Stdout); err != nil {
			return err
		}
		fmt.Println("Pronto. Reinicie o serviço: systemctl --user restart louvadeira")
		return nil
	case "logout":
		return wa.Logout(ctx)
	}
	return errors.New(usage)
}

func adminCommand(ctx context.Context, cfg config.Config, command string, args []string) error {
	flags := flag.NewFlagSet("admin "+command, flag.ContinueOnError)
	name := flags.String("name", "", "nome da pessoa")
	phone := flags.String("phone", "", "telefone com DDD")
	if err := flags.Parse(args); err != nil {
		return err
	}
	db, err := storage.Open(ctx, cfg.AppDBPath())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		return err
	}
	registry := store.New(db)

	switch command {
	case "add":
		person, err := registry.EnsureAdmin(ctx, store.PersonInput{Name: *name, Phone: *phone})
		if err != nil {
			return err
		}
		fmt.Printf("%s agora é administrador.\n", person.Name)
		return nil
	case "link":
		person, err := registry.PersonByPhone(ctx, *phone)
		if err != nil {
			return fmt.Errorf("pessoa com telefone %q: %w", *phone, err)
		}
		if !person.IsAdmin {
			return fmt.Errorf("%s não é administrador; rode 'louvadeira admin add' antes", person.Name)
		}
		token, err := registry.CreateLoginToken(ctx, person.ID, time.Now(), loginLinkTTL)
		if err != nil {
			return err
		}
		fmt.Printf("Link de acesso de %s (vale 15 minutos, uma vez):\n%s/entrar?token=%s\n", person.Name, cfg.PublicURL, token)
		return nil
	}
	return errors.New(usage)
}

func addresses(raw []string) []messenger.Address {
	out := make([]messenger.Address, len(raw))
	for i, item := range raw {
		out[i] = messenger.Address(item)
	}
	return out
}
