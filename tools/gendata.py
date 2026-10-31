#!/usr/bin/env python3
"""
Builds the data files milehigh embeds (internal/data/*.gz) from the open data in tools/source.

  airports.tsv.gz   OpenFlights airports.dat (ODbL) + OurAirports extras (public domain, from Skyline)
  airlines.tsv.gz   OpenFlights airlines.dat (ODbL)
  planes.tsv.gz     OpenFlights planes.dat (ODbL)
  countries.tsv.gz  ISO code, English name, continent
  world.bin.gz      Natural Earth 1:50m countries (public domain) rasterised to a 0.25° grid

Run from the repository root:  python3 tools/gendata.py   (needs Pillow)
"""
import csv
import gzip
import io
import json
import os
import struct

from PIL import Image, ImageDraw

SRC = "tools/source"
OUT = "internal/data"


def rows(path):
    with open(os.path.join(SRC, path), encoding="utf-8") as f:
        yield from csv.reader(f)


def nn(v):
    return "" if v in (None, "\\N", "-", "N/A") else v.strip()


def write_gz(name, text):
    with gzip.GzipFile(os.path.join(OUT, name), "wb", mtime=0) as f:
        f.write(text.encode("utf-8"))


# Country names (as OpenFlights spells them) -> ISO codes.
name_to_iso = {}
for line in open(os.path.join(SRC, "country_codes.tsv"), encoding="utf-8"):
    p = line.rstrip("\n").split("\t")
    if len(p) == 2:
        name_to_iso[p[0].lower()] = p[1].upper()
iso_names = {}
for r in rows("countries.dat"):
    if len(r) >= 2 and nn(r[1]) and len(r[1]) == 2:
        name_to_iso.setdefault(r[0].lower(), r[1].upper())
        iso_names.setdefault(r[1].upper(), r[0])
for name, iso in name_to_iso.items():
    pass
continents = dict(l.rstrip("\n").split("\t") for l in open(os.path.join(SRC, "continents.tsv")) if "\t" in l)


def iso(country):
    c = (country or "").strip()
    if len(c) == 2 and c.isalpha():
        return c.upper()
    return name_to_iso.get(c.lower(), "")


# Airports: OpenFlights first, then the newer/extra airports win.
airports = {}
for r in rows("airports.dat"):
    if len(r) < 12:
        continue
    code = nn(r[4])
    if len(code) != 3:
        continue
    airports[code] = [code, nn(r[5]), r[1], r[2], iso(r[3]), r[6], r[7], nn(r[11])]
for line in open(os.path.join(SRC, "airports_extra.tsv"), encoding="utf-8"):
    c = line.rstrip("\n").split("\t")
    if len(c) < 8:
        continue
    airports[c[0]] = [c[0], c[1], c[2], c[3], iso(c[4]), c[5], c[6], c[7]]
out = io.StringIO()
for code in sorted(airports):
    a = airports[code]
    a[5] = "%.5f" % float(a[5])
    a[6] = "%.5f" % float(a[6])
    out.write("\t".join(x.replace("\t", " ") for x in a) + "\n")
write_gz("airports.tsv.gz", out.getvalue())

# Airlines: iata, icao, name, callsign, country ISO, active.
out = io.StringIO()
for r in rows("airlines.dat"):
    if len(r) < 8:
        continue
    iata, icao = nn(r[3]), nn(r[4])
    if not iata and not icao:
        continue
    out.write("\t".join([iata, icao, r[1].strip(), nn(r[5]), iso(r[6]), r[7]]) + "\n")
# Well-known airlines whose codes OpenFlights also gives to defunct ones (from Skyline's seed): "S" wins.
for line in open(os.path.join(SRC, "airlines_seed.tsv"), encoding="utf-8"):
    c = line.rstrip("\n").split("\t")
    if len(c) == 4:
        out.write("\t".join([c[1], c[2], c[0], "", c[3], "S"]) + "\n")
write_gz("airlines.tsv.gz", out.getvalue())

# Aircraft types: name, iata, icao.
out = io.StringIO()
for r in rows("planes.dat"):
    if len(r) >= 3:
        out.write("\t".join([r[0], nn(r[1]), nn(r[2])]) + "\n")
write_gz("planes.tsv.gz", out.getvalue())

# Countries: iso, name, continent.
with gzip.open(os.path.join(SRC, "countries.geojson.gz"), "rt", encoding="utf-8") as f:
    geo = json.load(f)
# Names for countries OpenFlights lacks (microstates, newer codes) or spells in an older way.
for line in open(os.path.join(SRC, "country_names.tsv"), encoding="utf-8"):
    p = line.rstrip("\n").split("\t")
    if len(p) == 2:
        iso_names[p[0]] = p[1]
codes = sorted({ft["properties"]["iso"] for ft in geo["features"]} | set(continents) | set(iso_names))
out = io.StringIO()
for c in codes:
    out.write("\t".join([c, iso_names.get(c, c), continents.get(c, "")]) + "\n")
write_gz("countries.tsv.gz", out.getvalue())

# World raster: one byte per 0.25° cell, 0 = sea, n = index+1 into the header's ISO list.
W, H, S = 1440, 720, 4  # rasterise at 4x and take the majority-free "any land" sample at the centre
index = {c: i + 1 for i, c in enumerate(codes)}
img = Image.new("L", (W * S, H * S), 0)
draw = ImageDraw.Draw(img)


def ring(coords):
    return [((lon + 180) / 360 * W * S, (90 - lat) / 180 * H * S) for lon, lat in coords]


for ft in geo["features"]:
    g = ft["geometry"]
    val = index[ft["properties"]["iso"]]
    polys = g["coordinates"] if g["type"] == "MultiPolygon" else [g["coordinates"]]
    for poly in polys:
        draw.polygon(ring(poly[0]), fill=val)
        for hole in poly[1:]:
            draw.polygon(ring(hole), fill=0)
img = img.resize((W, H), Image.NEAREST)
header = ",".join(codes).encode()
with gzip.GzipFile(os.path.join(OUT, "world.bin.gz"), "wb", mtime=0) as f:
    f.write(struct.pack("<HHH", W, H, len(header)) + header + img.tobytes())
print("airports", len(airports), "countries", len(codes))
