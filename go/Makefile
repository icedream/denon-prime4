TRIPLET=x86_64-linux-gnu
CC=$(TRIPLET)-gcc
CXX=$(TRIPLET)-g++
PREFIX=/usr
WINDOWS_AMD64_TRIPLET=x86_64-w64-mingw32
WINDOWS_AMD64_PREFIX=/usr/$(WINDOWS_AMD64_TRIPLET)
WINDOWS_386_TRIPLET=i686-w64-mingw32
WINDOWS_386_PREFIX=/usr/$(WINDOWS_386_TRIPLET)
GOTAGS=libxz
LINUX_AMD64_TRIPLET=x86_64-linux-gnu
GO=go
GOOS=
GOARCH=
GOBUILDFLAGS=
CGO_ENABLED=1
DLLEXT=.so
BINEXT=
LINUX_DLL_BLACKLIST="^(libc.so)([\.\d+]+)\$$"
WINDOWS_DLL_BLACKLIST="^(advapi32.dll|comctl32.dll|comdlg32.dll|gdi32.dll|kernel32.dll|gdi32.dll|gdiplus.dll|glu32.dll|kernel32.dll|msvcrt.dll|ole32.dll|opengl32.dll|shell32.dll|user32.dll|ws2_32.dll)\$$"
DLL_BLACKLIST=$(WINDOWS_DLL_BLACKLIST)
LIB_PREFIX=$(PREFIX)/lib
BINS=find_update updater
GOGUIBUILDFLAGS=
GOBUILDLDFLAGS="-s -w"
GOBUILDFLAGS=

export CC CXX GOOS GOARCH CGO_ENABLED

all: $(addsuffix $(BINEXT),$(BINS))

#
# WINDOWS AMD64 BUILD CONVENIENCE TASKS
#

.PHONY: all-windows-amd64

all-windows-amd64:
	$(MAKE) \
		GOOS=windows \
		GOARCH=amd64 \
		TRIPLET=$(WINDOWS_AMD64_TRIPLET) \
		PREFIX=$(WINDOWS_AMD64_PREFIX) \
		LIB_PREFIX=$(WINDOWS_AMD64_PREFIX)/bin \
		GOGUIBUILDFLAGS='"-H windowsgui"' \
		BINEXT=.exe \
		DLLEXT=.dll \
		all \
		$(addprefix copy-libs-,$(addsuffix .exe,$(BINS)))

.PHONY: clean-windows-amd64

clean-windows-amd64:
	$(MAKE) \
		GOOS=windows \
		GOARCH=amd64 \
		TRIPLET=$(WINDOWS_AMD64_TRIPLET) \
		PREFIX=$(WINDOWS_AMD64_PREFIX) \
		LIB_PREFIX=$(WINDOWS_AMD64_PREFIX)/bin \
		GOGUIBUILDFLAGS="-H windowsgui" \
		BINEXT=.exe \
		DLLEXT=.dll \
		clean

#
# MAIN BINARY TARGETS
#

find_update$(BINEXT): go.mod go.sum $(wildcard ./pkg/**/*.go ./cmd/find_update/**/*.go)
	$(GO) build -tags $(GOTAGS) -ldflags $(GOBUILDLDFLAGS) $(GOBUILDFLAGS) -o $@ -v ./cmd/find_update

updater$(BINEXT): go.mod go.sum $(wildcard ./pkg/**/*.go ./cmd/updater/**/*.go)
	$(GO) build -tags $(GOTAGS) -ldflags $(GOGUIBUILDFLAGS)\ $(GOBUILDLDFLAGS) $(GOBUILDFLAGS) -o $@ -v ./cmd/updater

#
# PACKAGING TASKS
#

.PHONY: copy-libs-%

copy-libs-%: %
	$(MAKE) -s TARGET=$< copy-libs

.PHONY: copy-libs

# Extract list of DLL files that are used by imports.
#
# Windows output only.
copy-libs:
	$(TRIPLET)-objdump -p $(TARGET) | \
		grep -F "DLL Name:" | \
		sed -e "s/\t*DLL Name: //g" | \
		grep -Evi $(DLL_BLACKLIST) | \
		tr "\n" " " | \
		xargs -r -t $(MAKE)

%$(DLLEXT): $(LIB_PREFIX)/%$(DLLEXT)
	cp $< $@
	$(MAKE) -s TARGET=$@ copy-libs

clean:
	$(RM) $(wildcard updater$(BINEXT) *$(DLLEXT))
