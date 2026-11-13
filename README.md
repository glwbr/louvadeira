# louvadeira

Escalas dos ministérios da igreja: montar a escala, avisar pelo WhatsApp, confirmar presença e resolver ausências.

| Pasta | O que tem |
|---|---|
| `server/` | Go: API, SQLite, WhatsApp (whatsmeow) e o front embutido no binário |
| `web/` | PWA em Vite, React, TypeScript e shadcn (tema azul escuro) |
| `deploy/` | Serviço `systemd` de usuário, atalho `lv` e script de publicação no servidor |

## Desenvolvimento

```sh
mise install              # Go, Node e pnpm nas versões do mise.toml
mise run dev:server       # API em http://localhost:8090, dados em ./data
mise run dev:web          # front em http://localhost:5173, chamando a API local
mise run test             # testes do Go, typecheck e lint do front
```

Para entrar no app local, crie um administrador e gere um link:

```sh
cd server
LOUVADEIRA_DATA_DIR=../data go run ./cmd/louvadeira admin add -name "Nome" -phone "(11) 98888-7777"
LOUVADEIRA_DATA_DIR=../data go run ./cmd/louvadeira admin link -phone "(11) 98888-7777"
```

## Publicar no servidor

`mise.local.toml` (fora do git) define o destino:

```toml
[env]
LOUVADEIRA_HOST = "usuario@ip-do-servidor"
```

`mise run deploy` compila o front, gera o binário `linux/arm64`, copia para `~/louvadeira/bin/` e reinicia o
serviço. Na primeira vez cria `~/louvadeira/louvadeira.env` com o modo de teste e um token de administrador
aleatório.

Uma única vez no servidor, para o serviço continuar rodando sem sessão SSH aberta e voltar depois de reiniciar:

```sh
sudo loginctl enable-linger $USER
```

No servidor, `~/louvadeira/lv` roda o binário já com a configuração de `louvadeira.env` (rodar
`bin/louvadeira` direto usaria outra pasta de dados):

```sh
~/louvadeira/lv admin add -name "Nome" -phone "(11) 98888-7777"   # primeiro administrador
~/louvadeira/lv admin link -phone "(11) 98888-7777"               # link de acesso de 15 minutos
systemctl --user status louvadeira                                # estado
journalctl _SYSTEMD_USER_UNIT=louvadeira.service -f              # logs
cat ~/louvadeira/louvadeira.env                                   # configuração e token de administrador
```

## WhatsApp em modo de teste

No modo de teste o servidor só envia para os destinos em `LOUVADEIRA_WHATSAPP_ALLOWLIST`, ignora todas as outras
conversas, não baixa o histórico, não marca nada como lido e não aparece como online.

1. Vincular o número (no servidor, com o serviço parado):

   ```sh
   systemctl --user stop louvadeira
   ~/louvadeira/lv whatsapp link                       # QR Code no terminal
   ~/louvadeira/lv whatsapp link -phone 5511999999999  # ou código de 8 letras
   systemctl --user start louvadeira
   ```

2. Descobrir o ID do grupo de teste:

   ```sh
   curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8090/api/admin/whatsapp/groups
   ```

3. Colocar o ID do grupo em `LOUVADEIRA_WHATSAPP_ALLOWLIST` no `louvadeira.env` e reiniciar o serviço.

4. Mandar uma enquete de teste e votar pelo celular; o voto aparece nos logs:

   ```sh
   curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
     -d '{"to": "<id do grupo>"}' http://127.0.0.1:8090/api/admin/whatsapp/test-poll
   journalctl _SYSTEMD_USER_UNIT=louvadeira.service -f
   ```

Para desvincular: `~/louvadeira/lv whatsapp logout`, ou remover o aparelho "louvadeira" em
WhatsApp > Aparelhos conectados.
