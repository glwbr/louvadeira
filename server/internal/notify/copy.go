package notify

import (
	"fmt"
	"strings"

	"louvadeira/internal/store"
)

/**
 * NOTE: Every message the team reads is written here, in a warm and plain voice: no "votar", no "vaga", no orders.
 * The first line carries what matters (it is all a phone notification shows), *bold* marks the key fact in texts
 * (poll questions show it literally, so they stay plain), and each kind of message keeps its emoji: 🎶 schedule,
 * ✅ confirmed, 😕 cannot, 🔄 change, ⚠️ problem, ⏳ pending, 📅 absence, 🗓️ monthly round, 🍷 Lord's Supper.
 * WhatsApp answers a poll with a hash of the chosen label and each poll keeps the labels it was sent with, so
 * rewording an option only changes polls sent afterwards.
 */

const (
	optionGoing       = "✅ Vou estar lá"
	optionGoingBoth   = "✅ Vou nos dois"
	optionGoingAll    = "✅ Vou em todos"
	optionNotGoing    = "😕 Não vou conseguir"
	optionNotGoingAny = "😕 Não consigo em nenhum"
	optionCanCover    = "✅ Consigo"
	optionCannotCover = "😕 Dessa vez não"
	optionCoverNone   = "😕 Nenhum dessa vez"
)

func goingOption(services int) string {
	switch services {
	case 1:
		return optionGoing
	case 2:
		return optionGoingBoth
	}
	return optionGoingAll
}

func notGoingOption(date string) string {
	return fmt.Sprintf("😕 Não consigo %s (%s)", onWeekday(date), dayMonth(date))
}

/**
 * NOTE: "• Domingo, 15/11, às 19h · Voz": one service per line in every list.
 */
func serviceBullet(date, startTime string, roles []string) string {
	return fmt.Sprintf("• %s, %s, às %s · %s", capitalize(weekday(date)), dayMonth(date), hour(startTime), joinNames(roles))
}

func slotLine(d store.AssignmentDetail) string {
	return serviceBullet(d.Date, d.StartTime, []string{d.RoleName})
}

func confirmQuestion(name, ministry string, occurrences []occurrence) string {
	if len(occurrences) == 1 {
		o := occurrences[0]
		return fmt.Sprintf("🎶 %s · %s, %s, às %s · %s%s\nOi, %s! Você está na escala. Confirme sua presença:", ministry,
			capitalize(weekday(o.date)), dayMonth(o.date), hour(o.startTime), joinNames(o.roles), communionMark(o.communion), firstName(name))
	}
	days := make([]string, len(occurrences))
	lines := make([]string, len(occurrences))
	for i, o := range occurrences {
		days[i] = weekday(o.date) + ", " + dayMonth(o.date)
		lines[i] = serviceBullet(o.date, o.startTime, o.roles) + communionMark(o.communion)
	}
	return fmt.Sprintf("🎶 %s · %s\nOi, %s! Você está na escala:\n%s\nConfirme sua presença:", ministry, capitalize(joinNames(days)),
		firstName(name), strings.Join(lines, "\n"))
}

func confirmedReply(name string) string {
	return fmt.Sprintf("✅ *Confirmado*, %s! Obrigado 🙌\nSe surgir algum imprevisto, é só mudar sua resposta na enquete.", firstName(name))
}

const unforeseenHint = "Se surgir algum imprevisto, é só mudar sua resposta na enquete de confirmação."

func reminderText(d store.AssignmentDetail, roles []string) string {
	text := fmt.Sprintf("🎶 *Você está na escala de %s, %s!*\n%s", weekday(d.Date), firstName(d.PersonName),
		serviceBullet(d.Date, d.StartTime, roles))
	if d.Communion {
		text += "\n🍷 Dia de Santa Ceia"
	}
	return text + "\n" + unforeseenHint
}

/**
 * NOTE: The group's reminder of one service: one line per role marking who serves, leaving out who said they cannot
 * go, then the roles left with nobody. It returns the people to mark, once each.
 */
func lineupText(ministry store.MinistryDetail, sv store.ScheduleService) (string, []store.Person) {
	phones := map[int64]string{}
	for _, m := range ministry.Members {
		phones[m.PersonID] = m.Phone
	}
	var lines, missing []string
	var marked []store.Person
	seen := map[int64]bool{}
	for _, role := range sv.Roles {
		var names []string
		for _, slot := range role.Slots {
			if slot.PersonID == 0 || slot.Status == store.AssignmentDeclined {
				continue
			}
			person := store.Person{ID: slot.PersonID, Name: slot.PersonName, Phone: phones[slot.PersonID]}
			if name := mentionOf(person); !contains(names, name) {
				names = append(names, name)
			}
			if person.Phone != "" && !seen[person.ID] {
				seen[person.ID] = true
				marked = append(marked, person)
			}
		}
		if len(names) == 0 {
			missing = append(missing, role.Name)
			continue
		}
		lines = append(lines, fmt.Sprintf("• %s: %s", role.Name, joinNames(names)))
	}
	if len(missing) > 0 {
		lines = append(lines, "⚠️ Falta: "+joinNames(missing))
	}
	if sv.Communion {
		lines = append(lines, "🍷 Dia de Santa Ceia")
	}
	text := fmt.Sprintf("Paz, pessoal! 🎶 Escala do *%s* de %s:\n*%s*, %s, às %s\n\n%s\n\n%s", ministry.Name, weekday(sv.Date),
		sv.Name, dayMonth(sv.Date), hour(sv.StartTime), strings.Join(lines, "\n"), unforeseenHint)
	return text, marked
}

func pendingAlert(d store.AssignmentDetail, names []string) string {
	return fmt.Sprintf("⏳ *Falta responder* sobre %s (%s):\n%s", when(d), d.MinistryName, joinNames(names))
}

func declinedReply(name string, invited []string, toldLeaders bool, uncovered []string) string {
	text := fmt.Sprintf("😕 Tudo bem, %s, obrigado por avisar!", firstName(name))
	switch {
	case toldLeaders && len(invited) > 0:
		text += "\nJá avisei a liderança e chamei " + joinNames(invited) + " para ficar no seu lugar."
	case toldLeaders:
		text += "\nJá avisei a liderança."
	case len(invited) > 0:
		text += "\nJá chamei " + joinNames(invited) + " para ficar no seu lugar."
	}
	if len(uncovered) > 0 {
		text += "\n⚠️ Ninguém da equipe está livre para:\n" + strings.Join(uncovered, "\n") + "\nVai precisar ajustar a escala no app."
	}
	return text
}

func coverQuestion(candidate, absent string, d store.AssignmentDetail) string {
	return fmt.Sprintf("🔄 %s · %s, %s, às %s · %s\nOi, %s! %s não %s conseguir ir. Você consegue substituir?", d.MinistryName,
		capitalize(weekday(d.Date)), dayMonth(d.Date), hour(d.StartTime), d.RoleName, firstName(candidate), absent, goVerb(absent))
}

func coverManyQuestion(candidate, absent string, d store.AssignmentDetail, period string) string {
	if period == "" {
		return fmt.Sprintf("🔄 %s · substituição\nOi, %s! %s não %s conseguir ir nestes cultos. Em quais você consegue substituir? Pode marcar mais de um 🙂",
			d.MinistryName, firstName(candidate), absent, goVerb(absent))
	}
	return fmt.Sprintf("🔄 %s · %s\nOi, %s! %s não %s poder servir nesse período. Em quais destes cultos você consegue substituir? Pode marcar mais de um 🙂",
		d.MinistryName, period, firstName(candidate), absent, goVerb(absent))
}

// NOTE: "Ana não vai", "Ana e Bia não vão".
func goVerb(names string) string {
	if strings.Contains(names, " e ") {
		return "vão"
	}
	return "vai"
}

func coverOption(label string) string {
	return "✅ " + capitalize(label)
}

func declinedAlert(d store.AssignmentDetail, late bool, invited, failed []string) string {
	text := fmt.Sprintf("⚠️ *%s* não vai conseguir ir no %s\n%s", d.PersonName, d.MinistryName, slotLine(d))
	if late {
		text += "\n⏰ Avisou com menos de 72h"
	}
	if len(invited) > 0 {
		text += "\n🙋 Chamei " + joinNames(invited) + " para substituir"
	}
	if len(failed) > 0 {
		text += "\n📵 Não consegui avisar " + joinNames(failed) + " agora; vou tentar de novo em alguns minutos"
	}
	if len(invited) == 0 && len(failed) == 0 {
		text += "\n🙋 Ninguém da equipe está livre, vai precisar da sua ajuda"
	}
	return text
}

func declinedManyHeader(d store.AssignmentDetail, why absence) string {
	header := fmt.Sprintf("⚠️ *%s* não vai conseguir ir nestes cultos do %s", d.PersonName, d.MinistryName)
	if why.period != "" {
		header = fmt.Sprintf("⚠️ *%s* não vai poder servir no %s %s", d.PersonName, d.MinistryName, why.period)
	}
	if why.reason != "" {
		header += " (" + why.reason + ")"
	}
	return header
}

func declinedManyLine(d store.AssignmentDetail, late bool, invited, failed []string) string {
	line := slotLine(d) + ": " + inviteStatus(invited, failed)
	if late {
		line += " ⏰"
	}
	return line
}

func serviceHeaderLine(d store.AssignmentDetail, late bool) string {
	line := fmt.Sprintf("• %s, %s, às %s", capitalize(weekday(d.Date)), dayMonth(d.Date), hour(d.StartTime))
	if late {
		line += " ⏰"
	}
	return line
}

func roleSubLine(role string, invited, failed []string) string {
	return "   ◦ " + role + ": " + inviteStatus(invited, failed)
}

func inviteStatus(invited, failed []string) string {
	switch {
	case len(invited) > 0 && len(failed) > 0:
		return "chamei " + joinNames(invited) + "; vou tentar avisar " + joinNames(failed) + " de novo"
	case len(invited) > 0:
		return "chamei " + joinNames(invited)
	case len(failed) > 0:
		return "não consegui avisar " + joinNames(failed) + " agora, vou tentar de novo"
	}
	return "ninguém livre, vai precisar da sua ajuda"
}

func blockedNote(names []string) string {
	return "\n(Modo de teste: não chamei " + joinNames(names) + ".)"
}

func coveredReply(name string, lines []string) string {
	return fmt.Sprintf("✅ *Combinado*, %s! Obrigado 🙏\nVocê está na escala:\n%s", firstName(name), strings.Join(lines, "\n"))
}

func replacedNotice(absent, substitute string, lines []string) string {
	return fmt.Sprintf("🔄 *Boa notícia*, %s! %s vai ficar no seu lugar:\n%s", firstName(absent), substitute, strings.Join(lines, "\n"))
}

func leaderSwapNotice(absent, substitute string, lines []string) string {
	return fmt.Sprintf("🔄 Oi, %s! Mudança na escala: %s fica no seu lugar.\n%s", firstName(absent), substitute, strings.Join(lines, "\n"))
}

func removedNotice(name string, lines []string) string {
	return fmt.Sprintf("🔄 Oi, %s! Mudança na escala: você saiu deste culto.\n%s", firstName(name), strings.Join(lines, "\n"))
}

func swapAnnouncement(ministry, substitute, absent string, lines []string) string {
	return fmt.Sprintf("🔄 *Mudança na escala do %s*\n%s substitui %s\n%s", ministry, substitute, absent, strings.Join(lines, "\n"))
}

func alreadyTakenReply(name, substitute string, lines []string) string {
	return fmt.Sprintf("Obrigado pela disposição, %s! 🙏 %s já vai substituir:\n%s", firstName(name), substitute, strings.Join(lines, "\n"))
}

func goneReply(all bool, slots []string) string {
	switch {
	case all:
		return "Obrigado pela disposição! 🙏 Esse já foi resolvido."
	case len(slots) == 1:
		return fmt.Sprintf("Obrigado! 🙏 %s já foi resolvido.", capitalize(slots[0]))
	}
	return fmt.Sprintf("Obrigado! 🙏 %s já foram resolvidos.", capitalize(joinNames(slots)))
}

func alreadyServingReply(slots []string) string {
	return fmt.Sprintf("Você já está nesse culto (%s), então esse lugar fica para outra pessoa 🙂", joinNames(slots))
}

func refusedReply() string {
	return "Tudo bem, obrigado por responder! 🙏"
}

func uncoveredAlert(d store.AssignmentDetail, absent string, lines []string) string {
	return fmt.Sprintf("❗ *Ninguém conseguiu substituir %s* no %s\n%s\nVai precisar da sua ajuda nessa 🙏", absent, d.MinistryName, strings.Join(lines, "\n"))
}

func comebackTakenReply(d store.AssignmentDetail) string {
	return fmt.Sprintf("%s já ficou no seu lugar (%s). Se quiser voltar, fala com a liderança 🙂", d.PersonName, slotText(d))
}

func comebackReleased(name, returning string, d store.AssignmentDetail) string {
	return fmt.Sprintf("Não precisa mais, %s: %s vai conseguir ir (%s). Obrigado pela disposição! 🙏", firstName(name), returning, slotText(d))
}

func comebackAlert(returning string, d store.AssignmentDetail) string {
	return fmt.Sprintf("↩️ *%s* conseguiu ir, afinal (%s)\n%s", returning, d.MinistryName, slotLine(d))
}

func loginLinkText(name, link string) string {
	return fmt.Sprintf("🔑 Oi, %s! Aqui está seu link para ver suas escalas e avisar quando não puder ir:\n%s\nEle funciona uma vez e vale por 1 hora. Depois de entrar, o celular fica conectado.", firstName(name), link)
}

const (
	optionYesNotGoing  = "😕 Isso, não vou conseguir"
	optionMistakeGoing = "✅ Me enganei, vou sim"
	optionMistakeAll   = "✅ Me enganei, vou em todos"
	optionYesTell      = "😕 Isso, pode avisar"
	optionMistake      = "✅ Me enganei"
)

func helpText(name string) string {
	return fmt.Sprintf("🎶 *Assistente das escalas*\nOi, %s! Você pode me mandar:\n"+
		"• *escala*: para ver quando você está na escala\n"+
		"• *não posso dia 19*: para avisar que não vai conseguir ir num culto\n"+
		"• *não posso de 10/12 a 20/12*: para avisar que vai ficar fora um tempo\n"+
		"Para confirmar presença, é só responder as enquetes que eu mando, ou escrever *sim* ou *não* logo depois delas 🙂", firstName(name))
}

func scheduleText(lines []string) string {
	return "🎶 *Suas próximas escalas*\n" + strings.Join(lines, "\n") + "\nSe não puder ir em algum, é só me mandar *não posso* e o dia 🙂"
}

func noScheduleText(name string) string {
	return fmt.Sprintf("🎶 Oi, %s! Você não está em nenhuma escala publicada nos próximos dias 🙂", firstName(name))
}

func singleServiceQuestion(name string, s memberService) string {
	return fmt.Sprintf("😕 %s · %s, %s, às %s · %s\nOi, %s! Você não vai conseguir ir?", s.ministry,
		capitalize(weekday(s.date)), dayMonth(s.date), hour(s.startTime), joinNames(s.roles), firstName(name))
}

func whichServiceQuestion(name string) string {
	return fmt.Sprintf("😕 Oi, %s! Em qual culto você não vai conseguir ir? Pode marcar mais de um.", firstName(name))
}

func absenceQuestion(name, period string, lines []string) string {
	text := fmt.Sprintf("📅 Ausência %s\nSó para confirmar, %s: você não vai poder %s? Assim a gente não coloca você na escala nesses dias.",
		period, firstName(name), period)
	if len(lines) > 0 {
		text += "\nVocê está na escala em:\n" + strings.Join(lines, "\n") + "\nSe confirmar, eu já chamo alguém para ficar no seu lugar."
	}
	return text
}

func absenceDaysQuestion(name string, lines []string) string {
	text := fmt.Sprintf("📅 Ausência\nSó para confirmar, %s: em quais destes dias você não vai poder? Marque só os que estiverem certos 🙂", firstName(name))
	if len(lines) > 0 {
		text += "\nVocê está na escala em:\n" + strings.Join(lines, "\n")
	}
	return text
}

func absenceNotedReply(name, period string) string {
	return fmt.Sprintf("📅 *Anotado*, %s! A gente não coloca você na escala %s 🙏", firstName(name), period)
}

func absenceNotedAlert(name, period string) string {
	return fmt.Sprintf("📅 *%s* avisou que não vai poder %s.", name, period)
}

func keptReply() string {
	return "Tudo certo, nada mudou 🙂"
}

func alreadyToldReply() string {
	return "Eu já tinha avisado a liderança 🙂 Se mudou de ideia, fala com a liderança."
}

func pickInPollHint(accepting bool) string {
	if accepting {
		return "🙂 Para dizer em quais cultos você consegue, é só marcar na enquete que eu mandei."
	}
	return "🙂 Para dizer em qual culto você não consegue, é só marcar na enquete que eu mandei."
}

const (
	optionAvailableAll = "✅ Posso em todos"
	optionPublish      = "✅ Pode publicar"
	optionEdit         = "✏️ Vou ajustar no app"
)

func availabilityQuestion(name, ministry, month string) string {
	return fmt.Sprintf("🗓️ Escala de %s · %s\nOi, %s! Em quais destes cultos você não vai poder? Pode marcar mais de um. Se puder em todos, marque a última opção.",
		monthName(month), ministry, firstName(name))
}

func availabilityReply(name string, days []string) string {
	if len(days) == 0 {
		return fmt.Sprintf("🗓️ *Anotado*, %s! Você pode em todos 🙌", firstName(name))
	}
	return fmt.Sprintf("🗓️ *Anotado*, %s! Você não pode: %s.\nSe mudar algo, é só trocar a resposta 🙂", firstName(name), joinNames(days))
}

func availabilityReminder(name, month string) string {
	return fmt.Sprintf("🗓️ Oi, %s! Ainda falta você responder a enquete da escala de %s 🙂 É só marcar os cultos em que não vai poder, ou \"Posso em todos\", na enquete que eu mandei.",
		firstName(name), monthName(month))
}

func draftProposal(ministry, month, summary string, answered, asked, warnings int) string {
	text := fmt.Sprintf("🗓️ *Escala de %s do %s pronta*\n%d de %d responderam a enquete.", monthName(month), ministry, answered, asked)
	if warnings > 0 {
		text += fmt.Sprintf("\n⚠️ %d %s para revisar.", warnings, plural(warnings, "ponto", "pontos"))
	}
	return text + "\n\n" + summary
}

func approvalQuestion(month string) string {
	return fmt.Sprintf("🗓️ Posso publicar a escala de %s no grupo?", monthName(month))
}

func publishedReply(month string) string {
	return fmt.Sprintf("🎉 *Publicada!* Já mandei a escala de %s no grupo.", monthName(month))
}

func alreadyPublishedReply(month string) string {
	return fmt.Sprintf("A escala de %s já estava publicada 🙂", monthName(month))
}

func editReply() string {
	return "Combinado! Quando terminar os ajustes, é só publicar pelo app 🙂"
}

func draftFailedAlert(ministry, month string, err error) string {
	return fmt.Sprintf("⚠️ Não consegui montar a escala de %s do %s: %s.", monthName(month), ministry, err)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

const (
	optionGapKeep = "✅ Segue assim"
	optionGapAsk  = "📣 Pedir no grupo"
)

/**
 * NOTE: The leaders' question about a service still short of people, three days before it. Each line is one role:
 * how many it has, who cannot go and who is still free to step in.
 */
func gapQuestion(name string, gaps []store.ServiceGap) string {
	first := gaps[0]
	lines := make([]string, len(gaps))
	for i, g := range gaps {
		lines[i] = gapLine(g)
	}
	return fmt.Sprintf("⚠️ %s · %s, %s, às %s\nOi, %s! Ainda falta gente nesse culto:\n%s\nO que a gente faz?", first.MinistryName,
		capitalize(weekday(first.Date)), dayMonth(first.Date), hour(first.StartTime), firstName(name), strings.Join(lines, "\n"))
}

func gapLine(g store.ServiceGap) string {
	line := fmt.Sprintf("• %s: %d de %d", g.RoleName, g.Filled, g.Slots)
	if g.Filled == 0 {
		line = fmt.Sprintf("• %s: ninguém ainda", g.RoleName)
	}
	if g.Minimum < g.Slots {
		line += fmt.Sprintf(" (mínimo %d)", g.Minimum)
	}
	var notes []string
	if len(g.Away) > 0 {
		notes = append(notes, joinNames(g.Away)+" não "+plural(len(g.Away), "pode", "podem"))
	}
	notes = append(notes, joinNames(g.Free)+" "+plural(len(g.Free), "está livre", "estão livres"))
	return line + " · " + strings.Join(notes, "; ")
}

func gapKeptReply(date, startTime string) string {
	return fmt.Sprintf("✅ *Combinado!* O culto de %s segue assim. Não vou mais avisar sobre ele 🙂", whenAt(date, startTime))
}

func gapKeptAlert(leader, ministry, date, startTime string) string {
	return fmt.Sprintf("✅ %s decidiu que o culto de %s segue assim (%s).", leader, whenAt(date, startTime), ministry)
}

func gapEditReply() string {
	return "Combinado! É só ajustar a escala pelo app 🙂"
}

func gapAlreadyAskedReply() string {
	return "Já pedi no grupo 🙂 Quando alguém topar, a mudança aparece lá."
}

func gapCalledReply(ministry string) string {
	return fmt.Sprintf("📣 *Pronto!* Pedi no grupo do %s. Quem marcar primeiro fica com o lugar, e a mudança aparece lá.", ministry)
}

func gapCallFailedReply() string {
	return "😕 Não consegui mandar no grupo agora. Dá para ajustar a escala pelo app."
}

func gapClosedReply() string {
	return "✅ Não falta mais ninguém nesse culto 🙂"
}

/**
 * NOTE: "🙋 Eu posso (Voz)": the role goes in brackets so the label needs no "no/na" for it.
 */
func gapTakeOption(role string) string {
	return "🙋 Eu posso (" + role + ")"
}

func gapRoleName(label string) string {
	return strings.TrimSuffix(strings.TrimPrefix(label, "🙋 Eu posso ("), ")")
}

func gapCallQuestion(gaps []store.ServiceGap) string {
	first := gaps[0]
	roles := make([]string, len(gaps))
	for i, g := range gaps {
		roles[i] = g.RoleName
	}
	return fmt.Sprintf("🙋 %s · %s, %s, às %s\nAinda falta gente nesse culto: %s. Quem pode ajudar? Quem marcar primeiro fica com o lugar 🙏",
		first.MinistryName, capitalize(weekday(first.Date)), dayMonth(first.Date), hour(first.StartTime), joinNames(roles))
}

func gapFilledAnnouncement(ministry, volunteer, line string) string {
	return fmt.Sprintf("🙌 *Mudança na escala do %s*\n%s entra na escala\n%s", ministry, volunteer, line)
}

func outsiderReply(name string) string {
	return fmt.Sprintf("Obrigado, %s! 🙏 Avisei a liderança, que vai falar com você.", firstName(name))
}

func outsiderAlert(name, ministry, role, date, startTime string) string {
	return fmt.Sprintf("🙋 *%s* se ofereceu para %s no culto de %s, mas não está na equipe do %s. Dá para incluir pelo app.",
		name, role, whenAt(date, startTime), ministry)
}

func awayVolunteerReply(name string) string {
	return fmt.Sprintf("Obrigado, %s! 🙏 Você avisou que não pode nesse dia. Se mudou, fala com a liderança 🙂", firstName(name))
}
