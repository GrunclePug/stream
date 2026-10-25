# stream version
VERSION = 0.1.0

# paths
PREFIX = /usr/local

# xkbcommon's keysym header, the source of the keysym name table
KEYSYMS = /usr/include/xkbcommon/xkbcommon-keysyms.h

# flags
GOFLAGS = -trimpath
# -s -w: strip symbol table and DWARF; -X: inject version string
LDFLAGS = -s -w -X main.Version=${VERSION}

# tools
GO = go
RIVER = river
