#!/usr/bin/env python3
"""Helpers for drawing documentation diagrams as hand-placed SVG.

A definitions file imports these helpers, places every element by its coordinates, and calls ``write`` once per
diagram. The helpers carry what should not vary between diagrams: the palette, the text styles, the shapes, the
dark-mode rule and the accessibility markup. ``assets/example_diagrams.py`` shows one diagram of each kind.

Run a definitions file through ``diagrams.py draw`` to write its SVG files and check them. Copy this file next to the
definitions when a repository's contributors must redraw without this skill: it needs only the standard library.
"""

import html
import pathlib
import sys

"""CONSTRAINT: white text must reach a 4.5:1 contrast ratio on every fill, so containers use the darker C4 system
blue instead of #438DD5 and external elements use #6B7280 instead of #999999."""
LIGHT = {
    "bg": "#ffffff",
    "person": "#08427b",
    "container": "#1168bd",
    "component": "#85bbf0",
    "external": "#6b7280",
    "line": "#4b5563",
    "label": "#374151",
    "boundary": "#6b7280",
    "group": "#d1d5db",
    "group-label": "#6b7280",
}
DARK = {
    "bg": "#0d1117",
    "external": "#57606a",
    "line": "#9ca3af",
    "label": "#d1d5db",
    "boundary": "#8b949e",
    "group": "#30363d",
    "group-label": "#8b949e",
}
FONT = '-apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif'
RULES = """
    .canvas { fill: var(--bg); }
    .person { fill: var(--person); }
    .container { fill: var(--container); }
    .component { fill: var(--component); }
    .external { fill: var(--external); }
    .planned { fill: none; stroke: var(--container); stroke-width: 1.5; stroke-dasharray: 6 4; }
    .rim { fill: none; stroke: #ffffff; stroke-opacity: 0.55; stroke-width: 1.5; }
    .name { fill: #ffffff; font-size: 16px; font-weight: 700; text-anchor: middle; }
    .name.small { font-size: 14px; }
    .type { fill: #ffffff; fill-opacity: 0.85; font-size: 11px; text-anchor: middle; }
    .text { fill: #ffffff; font-size: 12.5px; text-anchor: middle; }
    .on-light { fill: #000000; fill-opacity: 1; }
    .on-canvas { fill: var(--label); fill-opacity: 1; }
    .boundary { fill: none; stroke: var(--boundary); stroke-width: 1.5; stroke-dasharray: 8 5; }
    .boundary-name { fill: var(--label); font-size: 14px; font-weight: 700; }
    .boundary-type { fill: var(--group-label); font-size: 11px; }
    .group { fill: none; stroke: var(--group); stroke-width: 1.25; }
    .group-label { fill: var(--group-label); font-size: 12px; font-weight: 600; letter-spacing: 0.04em; }
    .line { fill: none; stroke: var(--line); stroke-width: 1.5; }
    .line.dashed { stroke-dasharray: 6 4; }
    .arrowhead { fill: var(--line); }
    .label { fill: var(--label); font-size: 12px; paint-order: stroke; stroke: var(--bg); stroke-width: 5px; stroke-linejoin: round; }
    .label.tech { font-size: 11px; fill-opacity: 0.8; }
    .decision { fill: var(--bg); stroke: var(--line); stroke-width: 1.5; }
    .decision-text { fill: var(--label); font-size: 12.5px; text-anchor: middle; }
    .lifeline { fill: none; stroke: var(--group-label); stroke-width: 1.25; stroke-dasharray: 3 5; }
    .note { fill: var(--bg); stroke: var(--group-label); stroke-width: 1.25; }
    .note-text { fill: var(--label); font-size: 12px; text-anchor: middle; }
    .frame { fill: none; stroke: var(--boundary); stroke-width: 1.25; }
    .frame-title { fill: var(--label); font-size: 12px; font-weight: 700; }
    .dot { fill: var(--line); }
    .ring { fill: none; stroke: var(--line); stroke-width: 1.5; }
    .entity { fill: var(--bg); stroke: var(--container); stroke-width: 1.5; }
    .field { fill: var(--label); font-size: 12px; }
    .field.key { font-weight: 700; }
    .legend { fill: var(--label); font-size: 12px; }
"""

OUT = None
ONLY = ()
WRITTEN = []

_ADVANCE = {
    character: width
    for width, characters in (
        (191, "'"),
        (222, "ijl‘’"),
        (260, "|"),
        (278, " !,./:;I[\\]ft·"),
        (333, "()-`r“”"),
        (334, "{}"),
        (355, '"'),
        (389, "*"),
        (469, "^"),
        (500, "Jcksvxyz"),
        (556, "#$0123456789?L_abdeghnopqu–"),
        (584, "+<=>~"),
        (611, "FTZ"),
        (667, "&ABEKPSVXY"),
        (722, "CDHNRUw"),
        (778, "GOQ"),
        (833, "Mm"),
        (889, "%"),
        (944, "W"),
        (1000, "…—→"),
        (1015, "@"),
    )
    for character in characters
}


def text_width(text, size, weight=400):
    """Estimate how wide a line of text is drawn, in pixels.

    NOTE: the advance widths are Arial's. The factor widens them to about the 95th percentile of what Chrome measures
    for the macOS system font, the widest of the fonts the stack can resolve to, which sets small and bold text
    wider than Arial does.
    """
    wider = 1.05 if weight < 600 else 1.11 if weight < 700 else 1.14
    scale = max(1.0, wider + 0.011 * (16 - size))
    return sum(_ADVANCE.get(character, 600) for character in text) * size / 1000 * scale


def _n(value):
    return f"{value:g}"


def _escape(value):
    return html.escape(value, quote=False)


def _text(cls, x, y, value, anchor=None):
    place = f' text-anchor="{anchor}"' if anchor else ""
    return f'<text class="{cls}" x="{_n(x)}" y="{_n(y)}"{place}>{_escape(value)}</text>'


def _tone(kind):
    return {"component": " on-light", "planned": " on-canvas"}.get(kind, "")


def _element_text(kind, x, top, w, bottom, name, type_, lines, small=False):
    """The three kinds of line an element carries, centered between `top` and `bottom`: name, type, description."""
    size = 14 if small else 16
    gaps = ([18] if type_ else []) + [22 if order == 0 else 17 for order in range(len(lines))]
    height = 0.72 * size + sum(gaps) + 4
    baseline = top + (bottom - top - height) / 2 + 0.72 * size
    middle, tone = x + w / 2, _tone(kind)
    out = [_text(f"name{' small' if small else ''}{tone}", middle, baseline, name)]
    for cls, value, gap in zip(([f"type{tone}"] if type_ else []) + [f"text{tone}"] * len(lines), ([type_] if type_ else []) + list(lines), gaps):
        baseline += gap
        out.append(_text(cls, middle, baseline, value))
    return out


def box(kind, x, y, w, h, name, type_, lines=(), small=False):
    """An application, service or component: `kind` is container, component, external or planned."""
    return "\n".join([f'<rect class="{kind}" x="{_n(x)}" y="{_n(y)}" width="{_n(w)}" height="{_n(h)}" rx="10"/>', *_element_text(kind, x, y, w, y + h, name, type_, lines, small)])


def person(x, y, w, h, name, type_, lines=(), kind="person"):
    """A person: a head above a body that is `h` tall. Pass kind="external" for someone outside the system."""
    middle = x + w / 2
    return "\n".join(
        [
            f'<circle class="{kind}" cx="{_n(middle)}" cy="{_n(y + 22)}" r="22"/>',
            f'<rect class="{kind}" x="{_n(x)}" y="{_n(y + 42)}" width="{_n(w)}" height="{_n(h)}" rx="20"/>',
            *_element_text(kind, x, y + 50, w, y + 42 + h, name, type_, lines),
        ]
    )


def cylinder(x, y, w, h, name, type_, lines=(), kind="container"):
    """A data store."""
    half = w / 2
    return "\n".join(
        [
            f'<path class="{kind}" d="M{_n(x)},{_n(y + 12)} a{_n(half)},12 0 0 1 {_n(w)},0 v{_n(h - 24)} a{_n(half)},12 0 0 1 {_n(-w)},0 z"/>',
            f'<path class="rim" d="M{_n(x)},{_n(y + 12)} a{_n(half)},12 0 0 0 {_n(w)},0"/>',
            *_element_text(kind, x, y + 24, w, y + h - 6, name, type_, lines),
        ]
    )


def boundary(x, y, w, h, name, type_):
    """The dashed outline of a system, labeled at the bottom left."""
    return "\n".join(
        [
            f'<rect class="boundary" x="{_n(x)}" y="{_n(y)}" width="{_n(w)}" height="{_n(h)}" rx="12"/>',
            _text("boundary-name", x + 16, y + h - 30, name),
            _text("boundary-type", x + 16, y + h - 13, type_),
        ]
    )


def group(x, y, w, h, name):
    """A thin outline around the elements one host or layer holds, labeled at the top left."""
    return f'<rect class="group" x="{_n(x)}" y="{_n(y)}" width="{_n(w)}" height="{_n(h)}" rx="8"/>\n' + _text("group-label", x + 14, y + 20, name)


def line(path, dashed=False):
    """A line with no arrowhead, in SVG path syntax such as "M230,110 H330"."""
    return f'<path class="line{" dashed" if dashed else ""}" d="{path}"/>'


def arrow(path, dashed=False, both=False):
    """A relationship from the caller to the callee. Dashed is for what happens later: a queued job, a webhook, a reply."""
    start = ' marker-start="url(#arrow)"' if both else ""
    return f'<path class="line{" dashed" if dashed else ""}" d="{path}" marker-end="url(#arrow)"{start}/>'


def label(x, y, value, anchor="start", tech=False):
    """The words beside a line. `tech` is the protocol in brackets, on the line below."""
    return _text(f"label{' tech' if tech else ''}", x, y, value, None if anchor == "start" else anchor)


def participant(kind, x, w, name, type_, bottom, top=20):
    """One party to a sequence: its box at the top and its lifeline down to `bottom`."""
    middle, tone = x + w / 2, _tone(kind)
    return "\n".join(
        [
            f'<path class="lifeline" d="M{_n(middle)},{_n(top + 54)} V{_n(bottom)}"/>',
            f'<rect class="{kind}" x="{_n(x)}" y="{_n(top)}" width="{_n(w)}" height="54" rx="10"/>',
            _text(f"name small{tone}", middle, top + 23, name),
            _text(f"type{tone}", middle, top + 40, type_),
        ]
    )


def message(y, x_from, x_to, value, dashed=False, tech=None, at=None):
    """A call or an answer between two lifelines, with its words above the line.

    `at` moves the words off a lifeline the line passes over.
    """
    middle = at if at is not None else (x_from + x_to) / 2
    out = [arrow(f"M{_n(x_from)},{_n(y)} H{_n(x_to - 2 if x_to > x_from else x_to + 2)}", dashed), label(middle, y - 8, value, "middle")]
    if tech:
        out.append(label(middle, y + 15, tech, "middle", True))
    return "\n".join(out)


def note(x, y, w, h, lines):
    """A remark in a sequence, such as the commit that makes work visible to another party."""
    out = [f'<rect class="note" x="{_n(x)}" y="{_n(y)}" width="{_n(w)}" height="{_n(h)}" rx="6"/>']
    first = y + h / 2 + 4 - (len(lines) - 1) * 8
    out += [_text("note-text", x + w / 2, first + order * 16, value) for order, value in enumerate(lines)]
    return "\n".join(out)


def frame(x, y, w, h, title):
    """An outline around the messages of one phase of a sequence, titled on a tab at the top left."""
    tab = text_width(title, 12, 700) + 20
    return "\n".join(
        [
            f'<rect class="frame" x="{_n(x)}" y="{_n(y)}" width="{_n(w)}" height="{_n(h)}"/>',
            f'<path class="frame" d="M{_n(x)},{_n(y + 22)} H{_n(x + tab - 8)} l8,-8 V{_n(y)}"/>',
            _text("frame-title", x + 8, y + 15, title),
        ]
    )


def step(kind, x, y, w, h, lines, bold_first=False):
    """A box that holds only words: a flowchart's steps and outcomes, or a state with its name in bold."""
    tone = _tone(kind)
    out = [f'<rect class="{kind}" x="{_n(x)}" y="{_n(y)}" width="{_n(w)}" height="{_n(h)}" rx="10"/>']
    first = y + h / 2 + 4.5 - (len(lines) - 1) * 8.5
    for order, value in enumerate(lines):
        out.append(_text(f"{'name small' if bold_first and order == 0 else 'text'}{tone}", x + w / 2, first + order * 17, value))
    return "\n".join(out)


def decision(cx, cy, w, h, lines):
    """A diamond that holds a question. Label every line out of it with the answer."""
    points = f"{_n(cx)},{_n(cy - h / 2)} {_n(cx + w / 2)},{_n(cy)} {_n(cx)},{_n(cy + h / 2)} {_n(cx - w / 2)},{_n(cy)}"
    first = cy + 4 - (len(lines) - 1) * 8.5
    return "\n".join([f'<polygon class="decision" points="{points}"/>', *(_text("decision-text", cx, first + order * 17, value) for order, value in enumerate(lines))])


def start(cx, cy):
    """Where a flow or a record's life begins."""
    return f'<circle class="dot" cx="{_n(cx)}" cy="{_n(cy)}" r="8"/>'


def end(cx, cy):
    """A final state."""
    return f'<circle class="ring" cx="{_n(cx)}" cy="{_n(cy)}" r="11"/>\n<circle class="dot" cx="{_n(cx)}" cy="{_n(cy)}" r="6"/>'


def entity_height(fields):
    return 34 + len(fields) * 19 + 8


def entity(x, y, w, name, fields):
    """A record and its fields. Start a field with * to set it in bold, for a key or a field a relation hangs on."""
    out = [
        f'<rect class="entity" x="{_n(x)}" y="{_n(y)}" width="{_n(w)}" height="{_n(entity_height(fields))}" rx="8"/>',
        f'<path class="container" d="M{_n(x)},{_n(y + 34)} v-26 a8,8 0 0 1 8,-8 h{_n(w - 16)} a8,8 0 0 1 8,8 v26 z"/>',
        _text("name small", x + w / 2, y + 23, name),
    ]
    for order, field in enumerate(fields):
        out.append(_text(f"field{' key' if field.startswith('*') else ''}", x + 12, y + 34 + 19 * (order + 1), field.lstrip("*")))
    return "\n".join(out)


def many(x, y, way):
    """The forked end of a relation, on the side that has many. `way` is the side the line arrives from."""
    dx, dy = {"left": (-12, 0), "right": (12, 0), "up": (0, -12), "down": (0, 12)}[way]
    if dx:
        return f'<path class="line" d="M{_n(x + dx)},{_n(y)} L{_n(x)},{_n(y - 7)} M{_n(x + dx)},{_n(y)} L{_n(x)},{_n(y + 7)}"/>'
    return f'<path class="line" d="M{_n(x)},{_n(y + dy)} L{_n(x - 7)},{_n(y)} M{_n(x)},{_n(y + dy)} L{_n(x + 7)},{_n(y)}"/>'


def one(x, y, way):
    """The barred end of a relation, on the side that has one."""
    dx, dy = {"left": (-8, 0), "right": (8, 0), "up": (0, -8), "down": (0, 8)}[way]
    if dx:
        return f'<path class="line" d="M{_n(x + dx)},{_n(y - 7)} V{_n(y + 7)}"/>'
    return f'<path class="line" d="M{_n(x - 7)},{_n(y + dy)} H{_n(x + 7)}"/>'


def legend(x, y, entries):
    """A key, left to right from `x`. Each entry is (kind, words): a fill kind, or "solid" or "dashed" for a line."""
    out, at = [], x
    for kind, words in entries:
        if kind in ("solid", "dashed"):
            out.append(line(f"M{_n(at)},{_n(y - 4)} H{_n(at + 32)}", kind == "dashed"))
            at += 40
        else:
            out.append(f'<rect class="{kind}" x="{_n(at)}" y="{_n(y - 11)}" width="16" height="14" rx="3"/>')
            at += 24
        out.append(_text("legend", at, y, words))
        at += text_width(words, 12) + 28
    return "\n".join(out)


def write(name, width, height, title, desc, parts):
    """Write one diagram next to the file that calls this.

    `title` names the diagram and `desc` states its relationships in prose, for a reader who cannot see it.
    """
    if ONLY and not any(word in name for word in ONLY):
        return
    light = "".join(f"\n      --{key}: {value};" for key, value in LIGHT.items())
    dark = "".join(f"\n        --{key}: {value};" for key, value in DARK.items())
    style = f"\n    svg {{{light}\n      font-family: {FONT};\n    }}\n    @media (prefers-color-scheme: dark) {{\n      svg {{{dark}\n      }}\n    }}{RULES}"
    body = "\n".join(parts)
    svg = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {_n(width)} {_n(height)}" width="{_n(width)}" height="{_n(height)}" role="img" aria-labelledby="title desc">
  <title id="title">{_escape(title)}</title>
  <desc id="desc">{_escape(desc)}</desc>
  <style>{style}  </style>
  <defs>
    <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
      <path class="arrowhead" d="M0,0 L10,5 L0,10 z"/>
    </marker>
  </defs>
  <rect class="canvas" width="{_n(width)}" height="{_n(height)}"/>
{body}
</svg>
"""
    folder = pathlib.Path(OUT) if OUT else pathlib.Path(sys._getframe(1).f_globals.get("__file__", ".")).resolve().parent
    target = folder / name
    target.write_text(svg, encoding="utf-8")
    WRITTEN.append(target)
    print(f"wrote {target}")
