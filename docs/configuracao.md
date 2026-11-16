# Configuração e comandos

As variáveis de ambiente, os comandos do binário, os arquivos de uma instalação e as rotas de administração do
louvadeira.

## Variáveis de ambiente

O servidor lê a configuração de variáveis `LOUVADEIRA_*`. Em uma instalação feita com `mise run deploy`, elas ficam
em `~/louvadeira/louvadeira.env`, e o serviço precisa ser reiniciado depois de uma mudança.

| Variável | O que define | Valores | Padrão |
| --- | --- | --- | --- |
| `LOUVADEIRA_ADDR` | Endereço e porta em que o servidor escuta | `host:porta` | `:8090` |
| `LOUVADEIRA_DATA_DIR` | Pasta dos bancos de dados | Um caminho | `data` |
| `LOUVADEIRA_PUBLIC_URL` | Endereço público do app, usado nos links de acesso | Uma URL, sem barra no fim | `http://localhost:8090` |
| `LOUVADEIRA_WHATSAPP_MODE` | Para quem o WhatsApp envia | `off`, `test` ou `live` | `test` |
| `LOUVADEIRA_WHATSAPP_ALLOWLIST` | Destinos liberados no modo `test` | Identificadores de grupo e números com código do país, separados por vírgula | Vazio |
| `LOUVADEIRA_ADMIN_TOKEN` | Token das rotas de administração | Um texto longo e aleatório | Vazio. Sem valor, as rotas recusam qualquer token |
| `LOUVADEIRA_TIMEZONE` | Fuso dos horários de envio e das datas | Um nome da base IANA, como `America/Manaus` | `America/Sao_Paulo` |

O arquivo `louvadeira.env` criado pelo script de instalação define `LOUVADEIRA_ADDR=127.0.0.1:8090`, para o servidor
aceitar conexões só da própria máquina, e gera um `LOUVADEIRA_ADMIN_TOKEN` aleatório. Com `LOUVADEIRA_ADDR=:8090` o servidor
aceita conexões de qualquer rede, em HTTP.

Os modos do WhatsApp:

| Modo | Comportamento |
| --- | --- |
| `off` | O servidor não abre a sessão do WhatsApp e nenhuma mensagem sai |
| `test` | Envia só para os destinos de `LOUVADEIRA_WHATSAPP_ALLOWLIST` |
| `live` | Envia para toda a equipe e para os grupos dos ministérios |

## Comandos

No servidor, rode os comandos pelo atalho `~/louvadeira/lv`, que carrega `louvadeira.env` antes de chamar o binário.
Chamar `bin/louvadeira` direto usaria a configuração padrão e outra pasta de dados.

| Comando | O que faz |
| --- | --- |
| `louvadeira serve` | Inicia o servidor. É o que o serviço roda |
| `louvadeira admin add -name <nome> -phone <telefone>` | Cadastra um administrador, ou torna administrador quem já tem esse telefone |
| `louvadeira admin link -phone <telefone>` | Imprime um link de acesso para um administrador, válido por 15 minutos e um uso |
| `louvadeira whatsapp link` | Vincula um número por QR Code |
| `louvadeira whatsapp link -phone <numero-com-55>` | Vincula um número por código de 8 caracteres |
| `louvadeira whatsapp logout` | Desvincula o número |

Os comandos `whatsapp` exigem o serviço parado. Administradores só são criados por `admin add`. O app não tem essa
opção, e não há comando para retirar o papel.

## Arquivos no servidor

| Caminho | O que é |
| --- | --- |
| `~/louvadeira/bin/louvadeira` | O binário |
| `~/louvadeira/lv` | O atalho que carrega a configuração |
| `~/louvadeira/louvadeira.env` | A configuração, com o token de administrador. Só o dono lê |
| `~/louvadeira/data/app.db` | Cadastros, escalas e sessões |
| `~/louvadeira/data/whatsapp.db` | A sessão do WhatsApp |
| `~/.config/systemd/user/louvadeira.service` | O serviço do `systemd` |

Cada banco pode ter ao lado arquivos `-wal` e `-shm`, que fazem parte dele.

## Tarefas do mise

| Tarefa | O que faz |
| --- | --- |
| `mise run dev:server` | Servidor local em `:8090`, com dados em `./data` |
| `mise run dev:web` | App web com recarga automática em `:5173`, chamando o servidor local |
| `mise run web:install` | Instala as dependências do app web. As tarefas que precisam dela já a chamam |
| `mise run test` | `go vet` e testes do servidor, checagem de tipos e lint do app web |
| `mise run web:build` | Compila o app web e copia para dentro do servidor |
| `mise run build` | Gera `bin/louvadeira` para a sua máquina |
| `mise run build:linux` | Gera `bin/louvadeira-linux-amd64` e `bin/louvadeira-linux-arm64` |
| `mise run deploy` | Compila e publica no servidor de `LOUVADEIRA_HOST` |

## Acesso

| Item | Valor |
| --- | --- |
| Link de acesso enviado pelo WhatsApp | 1 hora, um uso |
| Link de acesso gerado por `admin link` | 15 minutos, um uso |
| Sessão no navegador | 90 dias |
| Pedidos de link pela tela de entrada | 10 a cada 15 minutos, por visitante |
| Tentativas de abrir um link | 30 a cada 15 minutos, por visitante |

## Rotas de administração

Estas rotas aceitam a sessão de um administrador ou o cabeçalho `Authorization: Bearer <token>`, com o valor de
`LOUVADEIRA_ADMIN_TOKEN`. Servem para testar e operar o WhatsApp com `curl`. O token é aceito também pelo endereço
público, então guarde como uma chave mestra.

| Rota | Corpo | O que faz |
| --- | --- | --- |
| `GET /api/admin/whatsapp/groups` | | Lista os grupos do número, com identificador e nome |
| `GET /api/admin/whatsapp/groups/{grupo}/participants` | | Lista telefone e nome dos participantes de um grupo |
| `POST /api/admin/whatsapp/test-poll` | `{"to": "<destino>"}` | Envia uma enquete de teste. Os votos aparecem no log |
| `POST /api/admin/whatsapp/message` | `{"to": "<destino>", "text": "<texto>"}` | Envia um texto. O campo opcional `mentions` marca telefones em um grupo |
| `POST /api/admin/whatsapp/test-buttons` | `{"to": "<destino>"}` | Envia os testes de botões e listas |
| `POST /api/admin/confirmations` | `{"from": "<AAAA-MM-DD>", "to": "<AAAA-MM-DD>"}` | Pede agora, a qualquer hora, a confirmação dos cultos do período que acontecem nos próximos 7 dias, a quem ainda não foi perguntado |

`GET /api/health` é aberta e responde com o modo do WhatsApp e se o número está vinculado e conectado.

Voltar ao [início](../README.md).
