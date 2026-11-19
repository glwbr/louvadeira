#!/usr/bin/env python3
"""Os diagramas da documentação, desenhados à mão como SVG.

Cada função desenha um diagrama. Para redesenhar depois de mudar palavras ou coordenadas:

    python3 docs/assets/draw.py
"""

from diagram_kit import arrow, boundary, box, cylinder, frame, group, label, message, participant, person, write


def ciclo_do_mes():
    servidor, integrante, lider, grupo = 110, 350, 590, 830
    entre_integrante_e_lider, entre_lider_e_grupo = 470, 710
    fim = 640
    write(
        "ciclo-do-mes.svg",
        940,
        660,
        "O ciclo do mês",
        "A partir do dia 20, a louvadeira pergunta a cada integrante em quais cultos do mês seguinte ele não pode, "
        "e o integrante marca os dias. Dois dias depois a louvadeira lembra quem não respondeu. Três dias depois "
        "manda o rascunho ao líder, que aprova, e a escala do mês é publicada no grupo do ministério. Em cada "
        "semana da escala, a louvadeira pede confirmação ao integrante sete dias antes, recebe a resposta, manda "
        "um lembrete na véspera do culto e avisa o líder, três dias antes, sobre quem falta responder.",
        [
            participant("container", 20, 180, "louvadeira", "[Servidor]", fim),
            participant("person", 260, 180, "Integrante", "[Pessoa]", fim),
            participant("person", 500, 180, "Líder", "[Pessoa]", fim),
            participant("external", 740, 180, "Grupo do ministério", "[WhatsApp]", fim),
            frame(130, 96, 790, 304, "A partir do dia 20"),
            message(152, servidor, integrante, "Em quais cultos você não pode?"),
            message(196, integrante, servidor, "Marca os dias", dashed=True),
            message(240, servidor, integrante, "2 dias depois: lembrete"),
            message(284, servidor, lider, "3 dias depois: rascunho", at=entre_integrante_e_lider),
            message(328, lider, servidor, "Pode publicar", dashed=True, at=entre_integrante_e_lider),
            message(372, servidor, grupo, "Escala do mês publicada", at=entre_lider_e_grupo),
            frame(130, 420, 790, 204, "Em cada semana da escala"),
            message(476, servidor, integrante, "7 dias antes: confirme presença"),
            message(520, integrante, servidor, "Confirma, ou avisa que não vai", dashed=True),
            message(564, servidor, integrante, "Véspera: lembrete do culto"),
            message(608, servidor, lider, "3 dias antes: falta responder", at=entre_integrante_e_lider),
        ],
    )


def arquitetura():
    write(
        "arquitetura.svg",
        900,
        780,
        "Os contêineres da louvadeira",
        "A liderança monta e publica as escalas no app web, e o integrante vê as suas escalas nele. O app web chama "
        "a API do servidor louvadeira por HTTPS, através de um proxy ou túnel. O servidor lê e grava o banco app.db, "
        "guarda a sessão do WhatsApp no banco whatsapp.db e envia mensagens e recebe votos pelo WhatsApp, como "
        "aparelho vinculado. O integrante responde no WhatsApp.",
        [
            person(105, 6, 190, 96, "Liderança", "[Pessoa]", ["Monta e publica as escalas"]),
            person(375, 6, 190, 96, "Integrante", "[Pessoa]", ["Serve nos cultos"]),
            boundary(12, 200, 580, 564, "louvadeira", "[Sistema de software]"),
            box("container", 105, 230, 190, 100, "App web", "[Contêiner: React, PWA]", ["Painel da liderança", "e página do integrante"]),
            group(28, 410, 548, 296, "Servidor Linux"),
            box("container", 85, 440, 230, 110, "Servidor louvadeira", "[Contêiner: Go]", ["API, bot e agendador,", "com o app web embutido"]),
            cylinder(60, 600, 170, 96, "app.db", "[Contêiner: SQLite]", ["Cadastros e escalas"]),
            cylinder(330, 600, 190, 96, "whatsapp.db", "[Contêiner: SQLite]", ["Sessão do WhatsApp"]),
            box("external", 660, 440, 190, 110, "WhatsApp", "[Sistema externo]", ["Entrega mensagens", "e enquetes"]),
            arrow("M200,144 V228"),
            label(212, 168, "Usa o painel"),
            label(212, 183, "[HTTPS]", tech=True),
            arrow("M440,144 V280 H297"),
            label(452, 236, "Vê suas escalas"),
            label(452, 251, "[HTTPS]", tech=True),
            arrow("M500,144 V165 H755 V438"),
            label(767, 300, "Responde enquetes"),
            arrow("M200,330 V438"),
            label(212, 372, "Chama a API"),
            label(212, 387, "[HTTPS, por proxy ou túnel]", tech=True),
            arrow("M315,495 H658"),
            label(327, 485, "Envia mensagens e recebe votos"),
            label(327, 510, "[Aparelho vinculado]", tech=True),
            arrow("M150,550 V598"),
            label(138, 572, "Lê e grava", "end"),
            label(138, 587, "[SQL]", "end", True),
            arrow("M290,550 V574 H425 V598"),
            label(360, 566, "Guarda a sessão", "middle"),
            label(360, 590, "[SQL]", "middle", True),
        ],
    )


if __name__ == "__main__":
    ciclo_do_mes()
    arquitetura()
