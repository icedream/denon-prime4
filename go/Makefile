TRIPLET=x86_64-linux-gnu
CC=$(TRIPLET)-gcc
CXX=$(TRIPLET)-g++
OBJDUMP=objdump
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
CGO_ENABLED=1
DLLEXT=.so
BINEXT=
LINUX_DLL_BLACKLIST="^((libc|libresolv).so)([\.[:digit:]+]+)\$$"
WINDOWS_DLL_BLACKLIST="^(advapi32.dll|comctl32.dll|comdlg32.dll|gdi32.dll|kernel32.dll|gdi32.dll|gdiplus.dll|glu32.dll|msvcrt.dll|ole32.dll|opengl32.dll|shell32.dll|user32.dll|ws2_32.dll|api-ms-.+|.+.drv)\$$"
DLL_BLACKLIST=$(WINDOWS_DLL_BLACKLIST)"|"$(LINUX_DLL_BLACKLIST)
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

	mkdir -p fltk_build/$(shell go env GOOS)/$(shell go env GOARCH)
	cd fltk_build/$(shell go env GOOS)/$(shell go env GOARCH) && if [ -d fltk ]; then cd fltk && git fetch; else git clone https://github.com/fltk/fltk.git fltk; fi
	cd fltk_build/$(shell go env GOOS)/$(shell go env GOARCH)/fltk && git reset --hard && git clean -ffdx && git checkout $(shell grep -Po 'commit = "\K.+(?=")' $(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)/fltk-build.go)
	cd fltk_build/$(shell go env GOOS)/$(shell go env GOARCH)/fltk && [ $(shell go env GOOS) != windows ] || git apply $(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)/lib/fltk-1.4.patch
	gen="Unix Makefiles"; if [ $(shell go env GOOS) = windows ] && [ $(shell uname -s) = windows ]; then gen="MinGW Makefiles"; fi; \
	if [ $(shell go env GOOS) = windows ]; then sysname="Windows"; \
	elif [ $(shell go env GOOS) = darwin ]; then sysname="Darwin"; \
	elif [ $(shell go env GOOS) = linux ]; then sysname="Linux"; \
	else sysname=$(shell uname -s); fi; \
	proc=$(shell go env GOARCH); \
	if [ $(shell go env GOARCH) = amd64 ]; then proc=x86_64; fi; \
	cmake \
		-G "$$gen" \
		-S fltk_build/$(shell go env GOOS)/$(shell go env GOARCH)/fltk \
		-B fltk_build/$(shell go env GOOS)/$(shell go env GOARCH)/build \
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
	$(MAKE) -C fltk_build/$(shell go env GOOS)/$(shell go env GOARCH)/build all install
	
	cd $(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk) && (\
		echo "//go:build $(shell go env GOOS) && $(shell go env GOARCH)" &&\
		echo "" &&\
		echo "package fltk" &&\
		echo "" &&\
		echo "// #cgo $(shell go env GOOS),$(shell go env GOARCH) CXXFLAGS: -std=c++11" &&\
		if [ $(shell go env GOOS) = windows ]; then \
			echo '// #cgo $(shell go env GOOS),$(shell go env GOARCH) CPPFLAGS: -I$${SRCDIR}/include/$(shell go env GOOS)/$(shell go env GOARCH) -I$${SRCDIR}/include/$(shell go env GOOS)/$(shell go env GOARCH)/FL/images -D_LARGEFILE_SOURCE -D_LARGEFILE64_SOURCE -D_FILE_OFFSET_BITS=64' &&\
			echo '// #cgo $(shell go env GOOS),$(shell go env GOARCH) LDFLAGS: -mwindows $${SRCDIR}/lib/$(shell go env GOOS)/$(shell go env GOARCH)/libfltk_images.a $${SRCDIR}/lib/$(shell go env GOOS)/$(shell go env GOARCH)/libfltk_jpeg.a $${SRCDIR}/lib/$(shell go env GOOS)/$(shell go env GOARCH)/libfltk_png.a $${SRCDIR}/lib/$(shell go env GOOS)/$(shell go env GOARCH)/libfltk_z.a $${SRCDIR}/lib/$(shell go env GOOS)/$(shell go env GOARCH)/libfltk_gl.a -lglu32 -lopengl32 $${SRCDIR}/lib/$(shell go env GOOS)/$(shell go env GOARCH)/libfltk_forms.a $${SRCDIR}/lib/$(shell go env GOOS)/$(shell go env GOARCH)/libfltk.a -lgdiplus -lole32 -luuid -lcomctl32 -lws2_32 -lwinspool';\
		fi &&\
		echo 'import "C"' &&\
		echo ""\
	)>"cgo_$(shell go env GOOS)_$(shell go env GOARCH).go"

#
# WINDOWS AMD64 BUILD CONVENIENCE TASKS
#

.PHONY: all-windows-amd64

all-windows-amd64:
	$(MAKE) \
		GOOS=windows \
		GOARCH=amd64 \
		TRIPLET=$(WINDOWS_AMD64_TRIPLET) \
		OBJDUMP=$(WINDOWS_AMD64_TRIPLET)-objdump \
		PREFIX=$(WINDOWS_AMD64_PREFIX) \
		LIB_PREFIX=$(WINDOWS_AMD64_PREFIX)/bin \
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
		OBJDUMP=$(WINDOWS_AMD64_TRIPLET)-objdump \
		PREFIX=$(WINDOWS_AMD64_PREFIX) \
		LIB_PREFIX=$(WINDOWS_AMD64_PREFIX)/bin \
		BINEXT=.exe \
		DLLEXT=.dll \
		clean

#
# WINDOWS 386 BUILD CONVENIENCE TASKS
#

.PHONY: all-windows-386

all-windows-386:
	$(MAKE) \
		GOOS=windows \
		GOARCH=386 \
		TRIPLET=$(WINDOWS_386_TRIPLET) \
		OBJDUMP=$(WINDOWS_386_TRIPLET)-objdump \
		PREFIX=$(WINDOWS_386_PREFIX) \
		LIB_PREFIX=$(WINDOWS_386_PREFIX)/bin \
		BINEXT=.exe \
		DLLEXT=.dll \
		all \
		$(addprefix copy-libs-,$(addsuffix .exe,$(BINS)))

.PHONY: clean-windows-386

clean-windows-386:
	$(MAKE) \
		GOOS=windows \
		GOARCH=386 \
		TRIPLET=$(WINDOWS_386_TRIPLET) \
		OBJDUMP=$(WINDOWS_386_TRIPLET)-objdump \
		PREFIX=$(WINDOWS_386_PREFIX) \
		LIB_PREFIX=$(WINDOWS_386_PREFIX)/bin \
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
	$(OBJDUMP) -p $(TARGET) | \
		grep -E "NEEDED|DLL Name:" | \
		sed -e "s/\t*DLL Name: //g" -e "s/\s*NEEDED\s*//g"  | \
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
