import { Drum, Guitar, MicVocal, Music, Piano, Projector, SlidersVertical, type LucideIcon } from "lucide-react"

/**
 * NOTE: Role names are free text typed by each church, so the glyph is a best guess from common words and falls back
 * to a music note.
 */
const ROLE_ICONS: [RegExp, LucideIcon][] = [
  [/voz|vocal|cant|back/, MicVocal],
  [/viol|guitar|baixo|contrabaixo|ukulele|cavaco/, Guitar],
  [/bateria|percuss|cajon|cajón|batera/, Drum],
  [/teclad|piano|sintet|órg|org/, Piano],
  [/som|áudio|audio|mesa|técnic/, SlidersVertical],
  [/mídia|midia|proje|transmiss|câmera|camera|vídeo|video|slide/, Projector],
]

export function roleIcon(name: string): LucideIcon {
  const lower = name.toLowerCase()
  return ROLE_ICONS.find(([pattern]) => pattern.test(lower))?.[1] ?? Music
}
