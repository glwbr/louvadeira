# Conectar o WhatsApp

Com um número de WhatsApp vinculado, a louvadeira envia as enquetes e recebe as respostas. Você começa em modo de
teste, em que os envios só chegam aos destinos da lista de permissão, e passa para o modo ativo quando tudo estiver
funcionando.

> [!WARNING]
> A louvadeira usa um cliente não oficial, o que viola os termos do WhatsApp, e o número pode ser banido. Use um
> número só para a louvadeira, nunca o número pessoal de alguém.
> [Arquitetura](arquitetura.md#whatsapp-por-cliente-não-oficial) explica o risco e o que o projeto faz para
> reduzi-lo.

## Antes de começar

- O app no ar (veja [Abrir o app na internet com HTTPS](abrir-na-internet.md)).
- Um número de WhatsApp só para a louvadeira, com o WhatsApp aberto em um celular.
- Um grupo de teste em que esse número esteja, com você e mais uma ou duas pessoas.

## Vincular o número

Você vincula o número pelo servidor, com o serviço parado, porque o WhatsApp aceita uma conexão por vez na mesma
sessão.

1. Pare o serviço:

   ```sh
   systemctl --user stop louvadeira
   ```

2. Rode o comando de vínculo. O terminal mostra um QR Code.

   ```sh
   ~/louvadeira/lv whatsapp link
   ```

   Sem câmera, dá para vincular por um código de 8 caracteres. Passe o número com o 55 na frente, como
   `5511988887777`:

   ```sh
   ~/louvadeira/lv whatsapp link -phone <numero-com-55>
   ```

3. No celular em que o número está, abra **WhatsApp > Aparelhos conectados > Conectar aparelho** e leia o QR Code.
   Se você pediu o código, escolha **Conectar com número de telefone** e digite o que o terminal mostrou. O terminal
   responde `Vinculado. Terminando a configuração...`.

4. Inicie o serviço:

   ```sh
   systemctl --user start louvadeira
   ```

O servidor passa a aparecer na lista de aparelhos conectados do celular.

## Testar com um grupo

No modo de teste, a louvadeira só envia para os destinos de `LOUVADEIRA_WHATSAPP_ALLOWLIST`, a lista de permissão.
Os números entram nela com o 55 na frente, porque é assim que a louvadeira os guarda.

1. Carregue o token de administrador e liste os grupos do número:

   ```sh
   source ~/louvadeira/louvadeira.env
   curl -s -H "Authorization: Bearer $LOUVADEIRA_ADMIN_TOKEN" http://127.0.0.1:8090/api/admin/whatsapp/groups
   ```

   A resposta traz o nome e o identificador de cada grupo, que termina em `@g.us`.

2. Em `~/louvadeira/louvadeira.env`, ponha na lista o grupo de teste e os números que vão receber mensagens no
   privado, separados por vírgula:

   ```sh
   LOUVADEIRA_WHATSAPP_ALLOWLIST=<id-do-grupo>,<numero-com-55>
   ```

   Troque `<id-do-grupo>` pelo identificador do passo 1.

3. Reinicie o serviço:

   ```sh
   systemctl --user restart louvadeira
   ```

4. Envie uma enquete de teste para o grupo:

   ```sh
   curl -s -X POST -H "Authorization: Bearer $LOUVADEIRA_ADMIN_TOKEN" -H "Content-Type: application/json" \
     -d '{"to": "<id-do-grupo>"}' http://127.0.0.1:8090/api/admin/whatsapp/test-poll
   ```

5. Responda à enquete pelo celular e acompanhe o log. Cada voto aparece como `voto na enquete de teste`.

   ```sh
   journalctl _SYSTEMD_USER_UNIT=louvadeira.service -f
   ```

Com o teste funcionando, a tela **Início** do app mostra o WhatsApp como conectado e em modo de teste. Agora dá para
ensaiar o ciclo inteiro com o grupo de teste: cadastre a equipe, escolha esse grupo no ministério e publique uma
escala, seguindo [Montar a primeira escala](montar-a-primeira-escala.md).

Enquanto o modo de teste durar, quem está fora da lista não recebe nada, nem o link de acesso pedido na tela de
entrada. Para essas pessoas entrarem no app, gere o link com `~/louvadeira/lv admin link`, que serve para
administradores, ou ponha o número na lista.

## Passar para o modo ativo

No modo ativo, a louvadeira envia para toda a equipe cadastrada e para o grupo de cada ministério. Combine a data da
virada com a equipe antes de trocar o modo, e peça que todos salvem o número nos contatos.

1. Em **Ministérios**, edite cada ministério e escolha o grupo de verdade em **Grupo do WhatsApp**.
2. Em `~/louvadeira/louvadeira.env`, troque o modo:

   ```sh
   LOUVADEIRA_WHATSAPP_MODE=live
   ```

3. Reinicie o serviço:

   ```sh
   systemctl --user restart louvadeira
   ```

As confirmações e os lembretes que ficaram retidos no modo de teste são enviados agora, se ainda estiverem no
prazo. A rodada do mês não é refeita: se o dia 20 passou durante o modo de teste, gere e publique a escala do mês
seguinte pelo app.

## Trocar ou desvincular o número

Para desvincular, pare o serviço e encerre a sessão. Depois vincule o número novo com os passos de
[Vincular o número](#vincular-o-número).

```sh
systemctl --user stop louvadeira
~/louvadeira/lv whatsapp logout
```

Você também pode remover o servidor da lista de aparelhos conectados, no celular.

Depois de uma troca de número, as enquetes antigas ainda sem resposta deixam de contar, porque só o número que
criou uma enquete consegue ler os votos dela. Quem ainda não tinha respondido confirma pela própria página no app,
e os administradores acompanham pelo painel.

## Se der errado

| O que aparece | Causa | O que fazer |
| --- | --- | --- |
| `este servidor já está vinculado` | Já existe uma sessão | Rode `~/louvadeira/lv whatsapp logout` e vincule de novo |
| `tempo esgotado; rode o comando de novo` | O QR Code ou o código expirou | Repita o passo 2 de [Vincular o número](#vincular-o-número) |
| `WhatsApp não vinculado` no log | O serviço iniciou sem sessão | Vincule o número e reinicie |
| `{"error":"WhatsApp desconectado"}` | O número ainda não conectou, ou alguém removeu o aparelho no celular | Espere um minuto. Se continuar, vincule de novo |
| `destino fora da lista de permissão do modo de teste` | O destino não está em `LOUVADEIRA_WHATSAPP_ALLOWLIST` | Acrescente o grupo ou o número e reinicie |

Voltar ao [início](../README.md).
