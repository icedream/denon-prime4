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
WINDOWS_DLL_BLACKLIST="^(advapi32.dll|comctl32.dll|comdlg32.dll|gdi32.dll|kernel32.dll|gdi32.dll|gdiplus.dll|glu32.dll|kernel32.dll|msvcrt.dll|ole32.dll|opengl32.dll|shell32.dll|user32.dll|ws2_32.dll|api-ms-.+|.+.drv)\$$"
DLL_BLACKLIST=$(WINDOWS_DLL_BLACKLIST)
LIB_PREFIX=$(PREFIX)/lib
BINS=find_update updater
GOGUIBUILDFLAGS=
GOBUILDLDFLAGS="-s -w"
GOBUILDFLAGS=

export CC CXX GOOS GOARCH CGO_ENABLED

.PHONY: all

all: $(addsuffix $(BINEXT),$(BINS))

.PHONY: go-mod-download

go-mod-download:
	[ x$(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk) != x ] || $(GO) mod download -x

.PHONY: build-fltk

#
# FLTK BINARY BUILD
#
# Needed to fix up incompatibilities within mingw
#
build-fltk: go-mod-download
	chmod -R +w $(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)

	#cd $(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk) && go generate

	mkdir -p fltk_build
	cd fltk_build && if [ -d fltk ]; then cd fltk && git fetch; else git clone https://github.com/fltk/fltk.git fltk; fi
	cd fltk_build/fltk && git reset --hard && git clean -ffdx && git checkout tags/release-1.4.4
	cd fltk_build/fltk && [ $(shell go env GOOS) != windows ] || git apply $(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)/lib/fltk-1.4.patch
	gen="Unix Makefiles"; if [ $(shell go env GOOS) = windows ] && [ $(shell uname -s) = windows ]; then gen="MinGW Makefiles"; fi; \
	if [ $(shell go env GOOS) = windows ]; then sysname="Windows"; \
	elif [ $(shell go env GOOS) = darwin ]; then sysname="Darwin"; \
	elif [ $(shell go env GOOS) = linux ]; then sysname="Linux"; \
	else sysname=$(shell uname -s); fi; \
	proc=$(shell go env GOARCH); \
	if [ $(shell go env GOARCH) = amd64 ]; then proc=x86_64; fi; \
	cmake \
		-G "$$gen" \
		-S fltk_build/fltk \
		-B fltk_build/build \
		"-DCMAKE_SYSTEM_NAME=$$sysname" \
		"-DCMAKE_SYSTEM_PROCESSOR=$$proc" \
		"-DCMAKE_FIND_ROOT_PATH=$(PREFIX)" \
		-DCMAKE_BUILD_TYPE=Release \
		-DFLTK_BUILD_TEST=OFF \
		-DFLTK_BUILD_EXAMPLES=OFF \
		-DFLTK_BUILD_FLUID=OFF \
		-DFLTK_BUILD_HTML_DOCS=OFF \
		-DFLTK_BUILD_PDF_DOCS=OFF \
		-DFLTK_BUILD_FLTK_OPTIONS=OFF \
		-DFLTK_USE_SYSTEM_LIBJPEG=OFF \
		-DFLTK_USE_SYSTEM_LIBPNG=OFF \
		-DFLTK_USE_SYSTEM_ZLIB=OFF \
		-DFLTK_USE_SYSTEM_LIBDECOR=OFF \
		-DCMAKE_C_COMPILER=$(CC) \
		-DCMAKE_CXX_COMPILER=$(CXX) \
		-DCMAKE_INSTALL_PREFIX=$(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk) \
		-DCMAKE_INSTALL_INCLUDEDIR=include/$(shell go env GOOS)/$(shell go env GOARCH) \
		-DCMAKE_INSTALL_LIBDIR=lib/$(shell go env GOOS)/$(shell go env GOARCH) \
		-DFLTK_INCLUDEDIR=$(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)/include/$(shell go env GOOS)/$(shell go env GOARCH) \
		-DFLTK_LIBDIR=$(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)/lib/$(shell go env GOOS)/$(shell go env GOARCH)
	$(MAKE) -C fltk_build/build all install

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

updater$(BINEXT): go.mod go.sum $(wildcard ./pkg/**/*.go ./cmd/updater/**/*.go) build-fltk
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

.PHONY: clean-fltk

clean-fltk:
	$(RM) -r fltk_build

.PHONY: clean

clean: clean-fltk
	$(RM) $(wildcard updater$(BINEXT) *$(DLLEXT))
