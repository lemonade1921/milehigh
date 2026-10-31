<h1 align="center">✈ milehigh</h1>

<p align="center">
  Your flight log in the terminal, inspired by Flighty.<br>
  For macOS and Linux, and in sync with <a href="https://github.com/lemonade1921/skyline">Skyline</a> on Android.
</p>

<p align="center">
  <a href="https://github.com/lemonade1921/milehigh/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/lemonade1921/milehigh/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/lemonade1921/milehigh/releases"><img alt="Release" src="https://img.shields.io/github/v/release/lemonade1921/milehigh?include_prereleases"></a>
  <a href="LICENSE"><img alt="License: GPL-2.0-or-later" src="https://img.shields.io/badge/license-GPL--2.0--or--later-blue"></a>
</p>

```
 ✈ milehigh   1 Flights  2 Map  3 Stats  4 Passport                             7 flights
──────────────────────────────────────────────────────────────────────────────────────────
 UPCOMING · 2                          │ VY 1234  Vueling       IN THE AIR · lands in 15m
▌ Sat 31 Oct  BCN → PMI       VY 1234  │ Saturday 31 October 2026
▌ Barcelon…  In the air · lands in 15m │
  Sun 20 Dec  MAD → EZE       UX 7700  │  BCN  ━━━━━━━━━━━━━━━━━━━━━━━━━━✈┄┄┄┄┄┄┄┄┄┄  PMI
  Madrid – Buenos…  23:55 · in 50 days │  Barcelona                     Palma de Mallorca
                                       │              202 km · 55m · 15m left
 2026 · 2                              │
  Tue 14 Apr  HND → SYD         QF 25  │ DEPARTURE                 ARRIVAL
  Tokyo – Sy…  21:35 → 08:35 · 10h 00m │ Barcelona Internationa…   Palma De Mallorca Airp…
  Sat 11 Apr  LHR → HND         JL 44  │ 09:00                     09:55
  London – T…  19:00 → 15:50 · 12h 50m │ scheduled · UTC+1         scheduled · UTC+1
                                       │
 2025 · 3                              │ Aircraft  Airbus A320
  Sun 09 Mar  JFK → MAD       IB 6252  │ Seat      14A
  New York – …  19:00 → 08:15 · 8h 15m │
  Sun 02 Mar  LHR → JFK        BA 117  │ ⠄⠄⠄⠄⠄⠄⠄⠄⠄⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠁⠁⠁⠁⠁⠁⠅⠅⠅⠅⠅⠅⠅⠁
  London – Ne…  11:25 → 14:20 · 7h 55m │ ⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠁          ⠁
  Sun 02 Mar  BCN → LHR       VY 7820  │ ⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠄
  Barcelona –…  07:10 → 08:35 · 2h 25m │ ⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅
                                       │ ⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅BCN⠅⣤⠁
                                       │ ⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠁⠁   ⢹⡀
                                       │ ⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠁⠁      ⢳
                                       │ ⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠁⠁        ⠈✈⠄⠄  ⠁
                                       │ ⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅⠅          ⠁⠰PMI
```

## Features

- **Flights**: upcoming and past flights grouped by year, with a Flighty-style detail view —
  departure and arrival times in each airport's time zone, delays in green/amber/red, terminals
  and gates, a progress bar while you're in the air, and a great-circle route map drawn in braille.
- **Map**: every route you've flown on a dotted world map, visited countries shaded, by year or all
  time. Pan with the arrow keys, zoom with `+`/`-`.
- **Stats**: flights, distance (and laps of the Earth), time in the air, top airports, airlines,
  routes, countries, continents and aircraft — for all time or any year.
- **Passport**: every country you've visited, with a world map; add or remove countries by hand.
- **Add flights in seconds**: type a flight number and its route is looked up; type a city and it
  finds the airport; see which flights fly a route.
- **Live**: aircraft position from community ADS-B receivers (`l`) and the weather at both ends (`w`).
- **Optional schedules**: with your own free AeroDataBox key, refresh scheduled/estimated/actual
  times, terminals and gates (`r`).
- **Works offline**: airports, airlines, aircraft, country outlines and the route index are built in.
  Nothing leaves your machine unless you ask for live data. No account, no tracking.

## Skyline sync

milehigh uses **Skyline's backup format** (and its file layout), so one log moves freely between
your phone and your computers:

- **Phone → computer**: in Skyline, *Settings › Backup › Export*, then `milehigh import skyline-backup.json`
  (or press `i` in the app).
- **Computer → phone**: `milehigh export` (or `x`), then restore the file in Skyline.

A restore merges, exactly like Skyline does: new flights are added; flights you already have keep
your edits and only gain the details they were missing (boarding pass, seat, times…). Your name,
units and clock format travel with the backup, as do your visited-country edits. Skyline-only
settings (map style, theme, language, Live Update opt-outs…) and boarding passes are kept and
written back untouched, so nothing is lost on the round trip.

To keep several computers in sync, point milehigh at a synced folder (Syncthing, iCloud Drive,
Nextcloud…): `milehigh -data ~/Sync/milehigh` or `export MILEHIGH_HOME=~/Sync/milehigh`.

You can also import a **Flighty** CSV export (*Settings › Export flights* in Flighty).

## Install

### macOS (Homebrew)

```sh
brew tap lemonade1921/milehigh https://github.com/lemonade1921/milehigh
brew install milehigh
```

The formula builds from source with Go. For the latest development version: `brew install --HEAD milehigh`.
Homebrew on Linux works the same way.

### Arch Linux, CachyOS, Manjaro…

Download the `.pkg.tar.zst` for your architecture from the
[latest release](https://github.com/lemonade1921/milehigh/releases/latest) and install it:

```sh
sudo pacman -U milehigh-*-x86_64.pkg.tar.zst
```

Or build from source with the PKGBUILD:

```sh
git clone https://github.com/lemonade1921/milehigh && cd milehigh/packaging/arch && makepkg -si
```

### Fedora

Download the `.rpm` from the [latest release](https://github.com/lemonade1921/milehigh/releases/latest):

```sh
sudo dnf install ./milehigh-*.x86_64.rpm
```

`packaging/fedora/milehigh.spec` builds it from source (locally with `rpmbuild`, or on COPR).

### Anywhere else

Grab a binary from the releases page (Linux and macOS, x86-64 and ARM64), or with Go 1.24+:

```sh
go install github.com/lemonade1921/milehigh/cmd/milehigh@latest
```

## Use

```
milehigh                      open the app
milehigh import FILE          import a Skyline backup (.json) or a Flighty CSV export
milehigh export [FILE|-]      write a backup Skyline on Android can restore
milehigh add FLIGHT DATE [FROM TO]    e.g.  milehigh add "VY 1234" 2026-11-04
milehigh list [-year N]       list flights
milehigh stats [-year N]      print your stats
milehigh config [KEY [VALUE]] show or change settings (name, units, timeFormat, temp, scheduleKey)
milehigh path                 where the log is stored
```

In the app:

| Key | |
|---|---|
| `1`–`4`, `tab` | Flights, Map, Stats, Passport |
| `↑` `↓` / `j` `k` | move |
| `a` / `e` / `d` | add, edit, delete a flight |
| `l` | live position (adsb.lol) |
| `w` | weather at departure and arrival (Open-Meteo) |
| `r` | refresh times and gates (AeroDataBox, your key) |
| `/` | filter by code, airport, city, airline or year |
| `t` | jump to your next flight |
| `i` / `x` | import / export |
| `S` | settings |
| `?` | help |

Your log lives in `~/Library/Application Support/milehigh` on macOS and
`~/.local/share/milehigh` on Linux (`$XDG_DATA_HOME` respected): `flights.json` is the same file
Skyline keeps on the phone, and `preferences.json` holds settings in Skyline's backup format.

## Open source, all the way down

milehigh is GPL-2.0-or-later and built only on open software and open data:
[Bubble Tea, Bubbles and Lip Gloss](https://charm.sh) for the interface;
[OpenFlights](https://openflights.org/data) and [OurAirports](https://ourairports.com/data/) for
airports, airlines and aircraft; [Natural Earth](https://www.naturalearthdata.com/) for the map;
[Virtual Radar Server](https://github.com/vradarserver/standing-data) for routes;
[adsbdb](https://www.adsbdb.com/), [adsb.lol](https://adsb.lol/) and
[Open-Meteo](https://open-meteo.com/) for live data; [GoReleaser](https://goreleaser.com) for the
packages. AeroDataBox is the one proprietary service, and it's optional. See [NOTICE](NOTICE).

## Develop

```sh
go test ./...
go run ./cmd/milehigh
python3 tools/gendata.py      # rebuild the bundled data from tools/source (needs Pillow)
```

Releases: push a tag like `v0.2.0`. The Release workflow publishes binaries and packages and
updates the Homebrew formula, the PKGBUILD and the Fedora spec.
