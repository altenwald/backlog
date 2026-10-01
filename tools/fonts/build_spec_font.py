"""Build the static spec reading fonts from the Recursive variable font.

Instance: MONO=0.5 (semi-mono), CASL=0 (linear), CRSV=0.5 (cursive only when
slanted). Recursive's code ligatures live in 'dlig', which Fyne cannot enable,
so their lookups are merged into 'liga' (on by default in HarfBuzz).

Fyne only adds line spacing between paragraphs, so the extra leading inside
wrapped text (LEADING, in em) is built into the vertical metrics, split evenly
above and below the glyphs to keep text vertically centred.
"""
import sys
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer

src, outdir = sys.argv[1], sys.argv[2]
STYLES = {
    "Regular": dict(wght=400, slnt=0),
    "Bold": dict(wght=700, slnt=0),
    "Italic": dict(wght=400, slnt=-15),
    "BoldItalic": dict(wght=700, slnt=-15),
}
FAMILY = "Backlog Recursive Semimono"
LEADING = 0.25


def add_leading(font):
    extra = round(font["head"].unitsPerEm * LEADING / 2)
    hhea, os2 = font["hhea"], font["OS/2"]
    hhea.ascent += extra
    hhea.descent -= extra
    os2.sTypoAscender += extra
    os2.sTypoDescender -= extra
    os2.usWinAscent = max(os2.usWinAscent, hhea.ascent)
    os2.usWinDescent = max(os2.usWinDescent, -hhea.descent)


def freeze_dlig(font):
    gsub = font["GSUB"].table
    feats = gsub.FeatureList.FeatureRecord
    for srec in gsub.ScriptList.ScriptRecord:
        langsyses = [srec.Script.DefaultLangSys] + [l.LangSys for l in srec.Script.LangSysRecord]
        for ls in langsyses:
            if ls is None:
                continue
            idx = ls.FeatureIndex
            dlig = [i for i in idx if feats[i].FeatureTag == "dlig"]
            liga = [i for i in idx if feats[i].FeatureTag == "liga"]
            if not dlig or not liga:
                continue
            target = feats[liga[0]].Feature
            for i in dlig:
                for lk in feats[i].Feature.LookupListIndex:
                    if lk not in target.LookupListIndex:
                        target.LookupListIndex.append(lk)
            target.LookupListIndex.sort()
            target.LookupCount = len(target.LookupListIndex)


def rename(font, style):
    name = font["name"]
    sub = {"BoldItalic": "Bold Italic"}.get(style, style)
    for rec in list(name.names):
        if rec.nameID in (1, 2, 3, 4, 6, 16, 17, 21, 22, 25):
            name.removeNames(nameID=rec.nameID)
    name.setName(FAMILY, 1, 3, 1, 0x409)
    name.setName(sub, 2, 3, 1, 0x409)
    name.setName(f"{FAMILY} {sub}; derived from Recursive 1.085", 3, 3, 1, 0x409)
    name.setName(f"{FAMILY} {sub}", 4, 3, 1, 0x409)
    name.setName(f"BacklogRecursiveSemimono-{style}", 6, 3, 1, 0x409)
    if "STAT" in font:
        del font["STAT"]


for style, loc in STYLES.items():
    f = TTFont(src)
    inst = instancer.instantiateVariableFont(f, dict(MONO=0.5, CASL=0, CRSV=0.5, **loc), updateFontNames=False)
    freeze_dlig(inst)
    add_leading(inst)
    rename(inst, style)
    for t in ("DSIG",):
        if t in inst:
            del inst[t]
    out = f"{outdir}/RecursiveSemimono-{style}.ttf"
    inst.save(out)
    print(out)
