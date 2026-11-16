# Instalar em um servidor

No fim deste guia, a louvadeira roda como serviço em uma máquina Linux sua e volta sozinha quando a máquina reinicia.
Serve qualquer Linux de 64 bits com `systemd`, em x86_64 ou ARM64: uma VPS, um mini PC ou uma placa como a
Raspberry Pi com sistema de 64 bits.

## Antes de começar

No servidor:

- Linux de 64 bits com `systemd` e `curl` instalado.
- Um usuário comum com acesso por SSH usando chave. O serviço roda com esse usuário, sem `root`.
- Permissão de `sudo` para um único comando, no passo 3.

Na sua máquina:

- O repositório clonado e o [mise](https://mise.jdx.dev) instalado.
- As ferramentas do projeto, que você instala com `mise install` na raiz do repositório.

## Instalar

1. Na raiz do repositório, crie o arquivo `mise.local.toml` com o endereço do servidor. Ele fica fora do git.

   ```toml
   [env]
   LOUVADEIRA_HOST = "<usuario>@<servidor>"
   ```

   Troque `<usuario>` pelo usuário do SSH e `<servidor>` pelo endereço ou IP da máquina.

2. Rode o deploy:

   ```sh
   mise run deploy
   ```

   O comando compila o app web, gera o binário para a arquitetura do servidor, copia tudo para `~/louvadeira` e
   inicia o serviço. No fim ele mostra o estado do serviço e a resposta do health check:

   ```json
   {"ok":true,"whatsapp":{"mode":"test","linked":false,"connected":false}}
   ```

3. No servidor, faça o serviço continuar rodando depois que você sair do SSH. Só precisa fazer isso uma vez.

   ```sh
   sudo loginctl enable-linger $USER
   ```

4. No servidor, cadastre o primeiro administrador com o WhatsApp dele:

   ```sh
   ~/louvadeira/lv admin add -name "<nome>" -phone "<whatsapp-com-ddd>"
   ```

5. Se a igreja não segue o horário de Brasília, abra `~/louvadeira/louvadeira.env`, acrescente a linha
   `LOUVADEIRA_TIMEZONE=<fuso>` (por exemplo, `America/Manaus`) e reinicie o serviço:

   ```sh
   systemctl --user restart louvadeira
   ```

## Como conferir

Confira o estado do serviço. Ele deve aparecer como `active (running)`:

```sh
systemctl --user status louvadeira
```

Na primeira instalação o comando cria `~/louvadeira/louvadeira.env`, com o WhatsApp em modo de teste e um token de
administrador gerado aleatoriamente. O serviço escuta em `127.0.0.1:8090` e só aceita conexões da própria máquina.
Para abrir o app no celular, siga [Abrir o app na internet com HTTPS](abrir-na-internet.md).

## Atualizar para uma versão nova

Faça um backup, atualize o repositório na sua máquina e rode `mise run deploy` de novo. O comando troca o binário e
reinicia o serviço. O arquivo `louvadeira.env` e a pasta `data` ficam como estão, e as migrações do banco rodam
quando o serviço inicia.

## Fazer backup dos dados

Tudo o que a igreja cadastrou está em `~/louvadeira/data`. Para copiar com segurança, pare o serviço, copie a pasta
inteira e inicie o serviço de novo:

```sh
systemctl --user stop louvadeira
cp -a ~/louvadeira/data ~/louvadeira-copia-$(date +%F)
systemctl --user start louvadeira
```

Leve a cópia para outra máquina. Ela contém os telefones da equipe, os motivos das ausências e a sessão do
WhatsApp. Guarde-a em lugar seguro. Para restaurar, pare o serviço, ponha a pasta de volta em `~/louvadeira/data` e
inicie o serviço.

## Se der errado

| O que aparece | Causa | O que fazer |
| --- | --- | --- |
| `defina LOUVADEIRA_HOST` | Falta o arquivo `mise.local.toml`, ou ele está fora da raiz | Refaça o passo 1 |
| `servidor não suportado` | A máquina não é Linux de 64 bits em x86_64 ou ARM64 | Use um sistema de 64 bits ou outra máquina |
| `curl: command not found` no fim | O servidor não tem `curl` | Instale o `curl` e rode `mise run deploy` de novo |
| `Failed to connect to bus` | A sessão SSH não tem o `systemd` do usuário | Entre direto com o usuário do serviço, sem `su` nem `sudo -u` |
| O health check falha | O serviço não iniciou | Leia o log com o comando abaixo |

```sh
journalctl _SYSTEMD_USER_UNIT=louvadeira.service -n 50
```

Voltar ao [início](../README.md).
