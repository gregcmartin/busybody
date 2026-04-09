# Busybody

Find out how busy someone really is by checking their public calendar availability. Generates a busyness score (0-100) from Calendly, Cal.com, Zoom Scheduler, or Google Calendar.

## Quick Start

```
git clone https://github.com/gregcmartin/busybody.git
cd busybody
```

Then open with [Claude Code](https://docs.anthropic.com/en/docs/claude-code) and let it handle the rest:

```
claude
```

## Usage

```
./busybody -calendar calendly.com/someone/30min
./busybody -calendar cal.com/someone
./busybody -calendar scheduler.zoom.us/someone
./busybody -site startup.com
```
