# Abrir o app na internet com HTTPS

O app precisa de um endereço público com HTTPS, como `https://louvadeira.example.com`. Os links de acesso enviados
no WhatsApp apontam para esse endereço, e o celular só deixa instalar o app quando ele vem por HTTPS.

Há dois caminhos, e você escolhe um:

| Seu servidor | Caminho |
| --- | --- |
| Tem IP público e as portas 80 e 443 livres, como uma VPS | [Proxy reverso](#com-um-proxy-reverso) |
| Fica em casa ou na igreja, atrás de um roteador | [Túnel do Cloudflare](#com-um-túnel-do-cloudflare) |

Nos dois, a louvadeira continua a escutar só em `127.0.0.1:8090`, e quem atende a internet é o proxy ou o túnel.

## Antes de começar

- A louvadeira instalada (veja [Instalar em um servidor](instalar-no-servidor.md)).
- Um domínio seu. Para o túnel, o domínio precisa estar em uma conta do Cloudflare.

## Com um proxy reverso

O exemplo usa o [Caddy](https://caddyserver.com), que emite e renova o certificado HTTPS sozinho.

1. No painel do seu domínio, crie um registro DNS do tipo A para `louvadeira.example.com` com o IP do servidor.
2. Instale o Caddy no servidor, pelo pacote da sua distribuição.
3. Em `/etc/caddy/Caddyfile`, aponte o endereço para a louvadeira:

   ```caddyfile
   louvadeira.example.com {
       reverse_proxy 127.0.0.1:8090
   }
   ```

4. Recarregue o Caddy:

   ```sh
   sudo systemctl reload caddy
   ```

Troque `louvadeira.example.com` pelo seu endereço nos passos 1 e 3.

Outro proxy serve, desde que rode na mesma máquina e escreva os cabeçalhos `X-Forwarded-Proto` e
`X-Forwarded-For` com o endereço do visitante. A louvadeira usa o primeiro para marcar o cookie de sessão como
seguro e o segundo para aplicar o limite de tentativas a cada visitante. O Caddy escreve os dois por padrão.

## Com um túnel do Cloudflare

O servidor abre a conexão para fora, então nenhuma porta do roteador fica aberta.

1. Na sua máquina, com o [`cloudflared`](https://github.com/cloudflare/cloudflared/releases) instalado, autorize o
   domínio, crie o túnel e aponte o endereço para ele:

   ```sh
   cloudflared tunnel login
   cloudflared tunnel create louvadeira
   cloudflared tunnel route dns louvadeira louvadeira.example.com
   ```

   O segundo comando mostra o identificador do túnel e grava as credenciais em
   `~/.cloudflared/<id-do-tunel>.json`. O certificado da conta, criado pelo primeiro comando, fica só na sua
   máquina.

2. No servidor, crie as pastas, baixe o `cloudflared` da arquitetura dele para `~/.local/bin/cloudflared` e torne
   o arquivo executável:

   ```sh
   mkdir -p ~/.local/bin ~/.cloudflared
   chmod +x ~/.local/bin/cloudflared
   ```

3. Copie `~/.cloudflared/<id-do-tunel>.json` da sua máquina para o mesmo caminho no servidor.

4. No servidor, crie `~/.cloudflared/config.yml`:

   ```yaml
   tunnel: <id-do-tunel>
   credentials-file: /home/<usuario>/.cloudflared/<id-do-tunel>.json
   ingress:
     - hostname: louvadeira.example.com
       service: http://127.0.0.1:8090
     - service: http_status:404
   ```

5. Copie `deploy/cloudflared.service` do repositório para `~/.config/systemd/user/` no servidor e ligue o serviço:

   ```sh
   systemctl --user daemon-reload
   systemctl --user enable --now cloudflared
   ```

Troque `<id-do-tunel>` pelo identificador do passo 1, `<usuario>` pelo usuário do servidor e
`louvadeira.example.com` pelo seu endereço.

## Apontar a louvadeira para o endereço

1. No servidor, abra `~/louvadeira/louvadeira.env` e preencha o endereço público, sem barra no fim:

   ```sh
   LOUVADEIRA_PUBLIC_URL=https://louvadeira.example.com
   ```

2. Reinicie o serviço:

   ```sh
   systemctl --user restart louvadeira
   ```

## Como conferir

De qualquer máquina, o health check responde pelo endereço público:

```sh
curl https://louvadeira.example.com/api/health
```

Depois gere um link de acesso para o administrador e abra no celular:

```sh
~/louvadeira/lv admin link -phone "<whatsapp-com-ddd>"
```

O link vale por 15 minutos e funciona uma vez. Depois de entrar, o celular fica conectado por 90 dias. Com o
WhatsApp conectado e em modo ativo, os próximos links chegam por lá, pedidos na tela de entrada.

O próximo passo é [Conectar o WhatsApp](conectar-o-whatsapp.md).

## Se der errado

| O que aparece | Causa | O que fazer |
| --- | --- | --- |
| O navegador avisa que o certificado é inválido | O DNS ainda não aponta para o servidor, ou as portas 80 e 443 estão fechadas | Confira o registro DNS e o firewall, e leia `journalctl -u caddy` |
| Erro 502 pelo endereço público | A louvadeira está parada | Rode `systemctl --user status louvadeira` |
| Erro 1033 do Cloudflare | O túnel está desligado | Rode `systemctl --user status cloudflared` |
| O link de acesso abre `localhost` | `LOUVADEIRA_PUBLIC_URL` está vazio | Refaça [Apontar a louvadeira para o endereço](#apontar-a-louvadeira-para-o-endereço) |

Voltar ao [início](../README.md).
