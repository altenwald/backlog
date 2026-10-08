"""Render the installer artwork at 1x and 2x without external design assets."""

import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont


def render(path, scale):
    width, height = 720, 460
    image = Image.new("RGB", (width * scale, height * scale))
    draw = ImageDraw.Draw(image)
    for y in range(height * scale):
        t = y / (height * scale - 1)
        color = tuple(round(a + (b - a) * t) for a, b in zip((249, 251, 255), (232, 240, 251)))
        draw.line((0, y, width * scale, y), fill=color)

    def box(coords):
        return tuple(round(v * scale) for v in coords)

    def text(label, y, size, color, bold=False):
        name = "Arial Bold.ttf" if bold else "Arial.ttf"
        font = ImageFont.truetype(f"/System/Library/Fonts/Supplemental/{name}", size * scale)
        draw.text((width * scale / 2, y * scale), label, font=font, fill=color, anchor="mt")

    text("Backlog", 44, 36, "#182942", bold=True)
    text("A little order. A lot of progress.", 94, 16, "#61728A")

    # Quiet landing areas leave room for Finder's real, draggable icons and labels.
    for x in (190, 530):
        draw.rounded_rectangle(box((x - 91, 168, x + 91, 322)), radius=24 * scale,
                               fill="#FFFFFF", outline="#DCE5F1", width=scale)
    draw.line(box((318, 242, 399, 242)), fill="#5480BE", width=4 * scale)
    draw.line(box((385, 229, 399, 242, 385, 255)), fill="#5480BE", width=4 * scale,
              joint="curve")

    text("Drag Backlog to Applications", 359, 20, "#223D61", bold=True)
    text("Then open Backlog from Applications. You’re ready to go.", 393, 14, "#61728A")
    image.save(path, dpi=(72 * scale, 72 * scale))


if __name__ == "__main__":
    target = Path(sys.argv[1])
    target.parent.mkdir(parents=True, exist_ok=True)
    render(target, 1)
    render(target.with_name(target.stem + "@2x" + target.suffix), 2)
