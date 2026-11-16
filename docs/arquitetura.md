# Arquitetura

A louvadeira é um binário Go que serve a API, entrega o app web embutido e roda o bot do WhatsApp no mesmo processo.
Os dados ficam em dois arquivos SQLite ao lado dele. O binário se conecta a um número de WhatsApp como aparelho
vinculado, do mesmo jeito que o WhatsApp Web, e por isso consegue mandar mensagens no privado e no grupo sem custo
por mensagem.

![Diagrama de contêineres: a liderança usa o app web e a equipe responde no WhatsApp. O app web chama o servidor louvadeira por um proxy ou túnel HTTPS. O servidor lê e grava os bancos app.db e whatsapp.db e envia e recebe mensagens pelo WhatsApp como aparelho vinculado.](assets/arquitetura.svg)

## As partes do servidor

O código do servidor fica em `server/internal/`, com um pacote para cada responsabilidade.

| Pacote | O que faz |
| --- | --- |
| `httpserver` | Rotas da API, sessão por cookie, limite de tentativas e entrega do app web |
| `notify` | O bot: mensagens agendadas, reação aos votos, substituições e comandos no privado |
| `schedule` | O gerador da escala, sem acesso a banco |
| `store` | Consultas e regras que precisam de transação |
| `storage` | Abertura do SQLite e migrações |
| `messenger` | A interface de envio, a pausa entre mensagens e a lista de permissão do modo de teste |
| `whatsapp` | O cliente whatsmeow por trás da interface `messenger` |
| `phone` | A forma canônica dos números brasileiros |
| `config` | Leitura das variáveis `LOUVADEIRA_*` |
| `webui` | O app web compilado, embutido com `embed` |

O bot envia pelo WhatsApp só pela interface `Messenger`, que tem três envios: texto, texto com pessoas mencionadas e
enquete. O envio fica isolado atrás dela. O fluxo por enquetes e o vínculo do número, em `cmd/louvadeira` e
`httpserver`, dependem do cliente atual.

## WhatsApp por cliente não oficial

O servidor usa a biblioteca [whatsmeow](https://github.com/tulir/whatsmeow), que implementa o protocolo do WhatsApp
Web. Com ela a louvadeira manda no grupo do ministério e no privado de cada integrante, como a equipe já conversa,
sem custo por mensagem. A alternativa é a API oficial da Meta, que é paga.

O preço dessa escolha é o risco: usar um cliente não oficial viola os termos do WhatsApp, e o número pode ser
banido. O projeto reduz o risco assim:

- Use um número só para a louvadeira. Com um número pessoal, um banimento leva junto as conversas da pessoa.
- O volume é baixo e vai para quem conhece o número: a equipe cadastrada e o grupo do ministério.
- As mensagens do bot saem uma por vez, com uma pausa aleatória de 2 a 5 segundos entre elas.
- O servidor não baixa o histórico de conversas ao vincular, não marca mensagens como lidas e não se anuncia como
  online, então o celular do número continua a se comportar como antes.

Um aparelho vinculado cai quando o celular principal passa muito tempo sem abrir o WhatsApp. Abra o aplicativo nesse
celular de tempos em tempos e mantenha a linha ativa.

## Enquetes no lugar de botões

Toda interação é por enquete. Botões e listas não funcionam em clientes não oficiais: nos testes do projeto, o
servidor do WhatsApp recusou a lista e os botões não apareceram no iPhone. Enquetes funcionam no privado e no grupo,
e as pessoas já sabem responder.

O WhatsApp entrega o voto como o hash da opção escolhida, e só o número que criou a enquete consegue lê-lo. Disso
saem duas consequências:

- Cada enquete guarda as opções com que foi enviada. Mudar o texto de uma opção no código vale para as enquetes
  enviadas depois.
- Ao trocar o número vinculado, as enquetes antigas ainda sem resposta deixam de funcionar e precisam ser enviadas
  de novo.

## O que o servidor lê do número

O número pode ser usado também para conversas comuns. O servidor age sobre três coisas: votos nas enquetes que ele
mesmo enviou, comandos de texto no privado vindos de integrantes cadastrados, e respostas aos seus testes de botões.
Mensagens de grupos, listas de transmissão e canais não chegam ao bot, e o texto das mensagens não vai para o log.

## Modo de teste

O WhatsApp começa em modo de teste. Nesse modo, todo envio passa por uma lista de permissão, definida em
`LOUVADEIRA_WHATSAPP_ALLOWLIST`, antes de chegar ao cliente. Um erro em qualquer outra parte do código só alcança o
grupo de teste e os números da lista.

As confirmações e os lembretes de quem está fora da lista são pulados sem registro, e saem depois da passagem para
o modo ativo se ainda estiverem no prazo. A rodada do mês é diferente: o prazo dela corre desde o dia 20 mesmo com
os envios bloqueados, e o rascunho com o pedido de aprovação não é reenviado aos líderes. Se a rodada de um mês
passou no modo de teste, gere e publique a escala daquele mês pelo app.

## Dois bancos SQLite

| Arquivo | O que guarda |
| --- | --- |
| `app.db` | Cadastros, escalas, ausências, substituições, enquetes enviadas e sessões |
| `whatsapp.db` | A sessão do aparelho vinculado, mantida pelo whatsmeow |

Os arquivos são separados para que a sessão do WhatsApp possa ser refeita sem tocar nos dados da igreja. Os dois
usam o modo WAL, e o `app.db` trabalha com uma conexão só. Isso serializa o acesso, o que basta para uma igreja e
elimina erros de banco ocupado entre duas escritas.

O driver é o `modernc.org/sqlite`, em Go puro. Por isso o binário compila estático, com `CGO_ENABLED=0`, para
qualquer arquitetura a partir de qualquer máquina.

## Acesso sem senha

Ninguém tem senha. O integrante pede um link de acesso pelo WhatsApp, e o link vira uma sessão em cookie, válida por
90 dias. O banco guarda só o hash do link e da sessão, então uma cópia do banco não serve para entrar.

A tela de entrada responde do mesmo jeito para um número cadastrado e para um desconhecido, e as rotas abertas têm
limite de tentativas por visitante. Atrás de um proxy ou túnel, o visitante é identificado pela última entrada de
`X-Forwarded-For`, que é a que o proxy escreve, ou por `CF-Connecting-IP` quando esse cabeçalho não vem. Os dois são
aceitos só quando a conexão vem da própria máquina.

Para tarefas feitas com `curl`, as rotas de administração aceitam também o token de `LOUVADEIRA_ADMIN_TOKEN`. O
token vale pelo endereço público também, então guarde como uma chave mestra.
[Configuração e comandos](configuracao.md) lista as rotas e os prazos.

## Números brasileiros

Um número digitado sem `+`, com 10 ou 11 dígitos, é tratado como brasileiro e ganha o 55. Um número de outro país
precisa ser digitado com `+` e o código do país.

O WhatsApp ainda registra muitos celulares brasileiros antigos sem o nono dígito. O servidor guarda todo número com
o nono dígito e, na hora de enviar, pergunta ao WhatsApp qual identificador a conta usa. Montar o identificador a
partir do número falharia para essas contas.

Em grupos, o WhatsApp pode identificar quem votou por um identificador interno (LID) no lugar do telefone. O
servidor traduz esse identificador para o telefone com o mapeamento que o próprio WhatsApp guarda.

## Limites conhecidos

- Uma instalação atende uma igreja. Para outra igreja, instale outra cópia.
- Os textos estão em português do Brasil e ficam em `server/internal/notify/copy.go` e `web/src/locales/pt-BR.ts`.
- Os prazos das mensagens são constantes no código, iguais para todos os ministérios.
- Os administradores são criados e promovidos no servidor, pelo comando `admin add`. O app não tem essa opção, e
  não há comando para retirar o papel.
- Nada avisa quando o WhatsApp desconecta. O estado aparece na tela **Início** do painel, em `/api/health` e no log.
- O servidor e os comandos `louvadeira whatsapp` usam a mesma sessão, e o WhatsApp aceita uma conexão por vez para
  ela. Por isso o vínculo é feito com o serviço parado.

Voltar ao [início](../README.md).
