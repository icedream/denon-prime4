TRIPLET=x86_64-linux-gnu
CC=$(TRIPLET)-gcc
CXX=$(TRIPLET)-g++
OBJDUMP=objdump
PREFIX=/usr
WINDOWS_AMD64_TRIPLET=x86_64-w64-mingw32
WINDOWS_AMD64_PREFIX=/usr/$(WINDOWS_AMD64_TRIPLET)
WINDOWS_386_TRIPLET=i686-w64-mingw32
WINDOWS_386_PREFIX=/usr/$(WINDOWS_386_TRIPLET)
GOTAGS=libxz,osusergo,netgo,static_build
LINUX_AMD64_TRIPLET=x86_64-linux-gnu
GO=go
GOOS=$(shell $(GO) env GOOS)
GOARCH=$(shell $(GO) env GOARCH)
CGO_ENABLED=1
DLLEXT=.so
BINEXT=
LINUX_DLL_BLACKLIST="^((ld-linux-.+|libdbus*|libc|libm|libresolv).so)([\.[:digit:]+]+)\$$"
WINDOWS_DLL_BLACKLIST="^(advapi32.dll|comctl32.dll|comdlg32.dll|gdi32.dll|kernel32.dll|gdi32.dll|gdiplus.dll|glu32.dll|msvcrt.dll|ole32.dll|opengl32.dll|shell32.dll|user32.dll|ws2_32.dll|api-ms-.+|.+.drv)\$$"
DLL_BLACKLIST=$(WINDOWS_DLL_BLACKLIST)"|"$(LINUX_DLL_BLACKLIST)
LIB_PREFIX=$(PREFIX)/lib
BINS=find_update updater
BINSUFFIX=_$(GOOS)_$(GOARCH)
GOBUILDFLAGS=
GOBUILDLDFLAGS='-s -w -extldflags "-fno-PIC"'
GOBUILDFLAGS=-buildmode pie -v
GOBUILDGUIFLAGS=
GOBUILDGUILDFLAGS=

export CC CXX GOOS GOARCH CGO_ENABLED

.PHONY: all

all: $(addsuffix $(BINSUFFIX)$(BINEXT),$(BINS))

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
	if [ -z "$(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)" ]; then echo "ERROR: go-fltk dir not resolvable" >&2; exit 1; fi

	chmod -R +w $(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)

	#cd $(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk) && go generate

	mkdir -p fltk_build/$(GOOS)/$(GOARCH)
	cd fltk_build/$(GOOS)/$(GOARCH) && if [ -d fltk ]; then cd fltk && git fetch; else git clone https://github.com/fltk/fltk.git fltk; fi
	cd fltk_build/$(GOOS)/$(GOARCH)/fltk && git reset --hard && git clean -ffdx && git checkout $(shell grep -Po 'commit = "\K.+(?=")' $(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)/fltk-build.go)
	cd fltk_build/$(GOOS)/$(GOARCH)/fltk && [ $(GOOS) != windows ] || git apply $(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)/lib/fltk-1.4.patch
	gen="Unix Makefiles"; if [ $(GOOS) = windows ] && [ $(shell uname -s) = windows ]; then gen="MinGW Makefiles"; fi; \
	if [ $(GOOS) = windows ]; then sysname="Windows"; \
	elif [ $(GOOS) = darwin ]; then sysname="Darwin"; \
	elif [ $(GOOS) = linux ]; then sysname="Linux"; \
	else sysname=$(shell uname -s); fi; \
	proc=$(GOARCH); \
	if [ $(GOARCH) = amd64 ]; then proc=x86_64; fi; \
	cmake \
		-G "$$gen" \
		-S fltk_build/$(GOOS)/$(GOARCH)/fltk \
		-B fltk_build/$(GOOS)/$(GOARCH)/build \
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
		-DCMAKE_INSTALL_INCLUDEDIR=include/$(GOOS)/$(GOARCH) \
		-DCMAKE_INSTALL_LIBDIR=lib/$(GOOS)/$(GOARCH) \
		-DFLTK_INCLUDEDIR=$(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)/include/$(GOOS)/$(GOARCH) \
		-DFLTK_LIBDIR=$(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)/lib/$(GOOS)/$(GOARCH)
	$(MAKE) -C fltk_build/$(GOOS)/$(GOARCH)/build all install
	chmod +x ./fltk_build/$(GOOS)/$(GOARCH)/build/bin/fltk-config
	cd $(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk) && (\
		echo "//go:build $(GOOS) && $(GOARCH)" &&\
		echo "" &&\
		echo "package fltk" &&\
		echo "" &&\
		echo "// #cgo $(GOOS),$(GOARCH) CXXFLAGS: -std=c++11" &&\
		if [ $(GOOS) != windows ]; then \
			cxx_output="$$(\
				$(CURDIR)/fltk_build/$(GOOS)/$(GOARCH)/build/bin/fltk-config  --use-gl --use-images --use-forms --cxxflags |\
				sed -e 's#$(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)#$${SRCDIR}#g' \
			)" &&\
			echo '// #cgo $(GOOS),$(GOARCH) CPPFLAGS: '"$${cxx_output}" &&\
			ld_output="$$(\
				$(CURDIR)/fltk_build/$(GOOS)/$(GOARCH)/build/bin/fltk-config  --use-gl --use-images --use-forms --ldstaticflags |\
				sed -e 's#$(shell $(GO) list -m -f '{{.Dir}}' github.com/pwiecz/go-fltk)#$${SRCDIR}#g' \
			)" &&\
			echo '// #cgo $(GOOS),$(GOARCH) LDFLAGS: '"$${ld_output}" ;\
		else \
			echo '// #cgo $(GOOS),$(GOARCH) CPPFLAGS: -I$${SRCDIR}/include/$(GOOS)/$(GOARCH) -I$${SRCDIR}/include/$(GOOS)/$(GOARCH)/FL/images -D_LARGEFILE_SOURCE -D_LARGEFILE64_SOURCE -D_FILE_OFFSET_BITS=64' &&\
			echo '// #cgo $(GOOS),$(GOARCH) LDFLAGS: -mwindows $${SRCDIR}/lib/$(GOOS)/$(GOARCH)/libfltk_images.a $${SRCDIR}/lib/$(GOOS)/$(GOARCH)/libfltk_jpeg.a $${SRCDIR}/lib/$(GOOS)/$(GOARCH)/libfltk_png.a $${SRCDIR}/lib/$(GOOS)/$(GOARCH)/libfltk_z.a $${SRCDIR}/lib/$(GOOS)/$(GOARCH)/libfltk_gl.a -lglu32 -lopengl32 $${SRCDIR}/lib/$(GOOS)/$(GOARCH)/libfltk_forms.a $${SRCDIR}/lib/$(GOOS)/$(GOARCH)/libfltk.a -lgdiplus -lole32 -luuid -lcomctl32 -lws2_32 -lwinspool';\
		fi &&\
		echo 'import "C"' &&\
		echo ""\
	)>"cgo_$(GOOS)_$(GOARCH).go"

#
# WINDOWS AMD64 BUILD CONVENIENCE TASKS
#

.PHONY: copy-libs-windows-amd64

copy-libs-windows-amd64:
	$(MAKE) \
		GOOS=windows \
		GOARCH=amd64 \
		TRIPLET=$(WINDOWS_AMD64_TRIPLET) \
		OBJDUMP=$(WINDOWS_AMD64_TRIPLET)-objdump \
		PREFIX=$(WINDOWS_AMD64_PREFIX) \
		LIB_PREFIX=$(WINDOWS_AMD64_PREFIX)/bin \
		BINEXT=.exe \
		DLLEXT=.dll \
		$(addprefix copy-libs-,$(addsuffix _windows_amd64.exe,$(BINS)))

.PHONY: all-windows-amd64

all-windows-amd64:
	$(MAKE) \
		GOOS=windows \
		GOARCH=amd64 \
		TRIPLET=$(WINDOWS_AMD64_TRIPLET) \
		OBJDUMP=$(WINDOWS_AMD64_TRIPLET)-objdump \
		PREFIX=$(WINDOWS_AMD64_PREFIX) \
		LIB_PREFIX=$(WINDOWS_AMD64_PREFIX)/bin \
		'GOBUILDLDFLAGS="-s -w -extldflags \"-fno-PIC -static\""' \
		'GOBUILDGUILDFLAGS="-H windowsgui"' \
		BINEXT=.exe \
		DLLEXT=.dll \
		all \
		$(addprefix copy-libs-,$(addsuffix _windows_amd64.exe,$(BINS)))

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

.PHONY: copy-libs-windows-386

copy-libs-windows-386:
	$(MAKE) \
		GOOS=windows \
		GOARCH=386 \
		TRIPLET=$(WINDOWS_386_TRIPLET) \
		OBJDUMP=$(WINDOWS_386_TRIPLET)-objdump \
		PREFIX=$(WINDOWS_386_PREFIX) \
		LIB_PREFIX=$(WINDOWS_386_PREFIX)/bin \
		'GOBUILDLDFLAGS="-s -w -extldflags \"-fno-PIC -static\""' \
		'GOBUILDGUILDFLAGS="-H windowsgui"' \
		BINEXT=.exe \
		DLLEXT=.dll \
		$(addprefix copy-libs-,$(addsuffix _windows_386.exe,$(BINS)))

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
		$(addprefix copy-libs-,$(addsuffix _windows_386.exe,$(BINS)))

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

find_update$(BINSUFFIX)$(BINEXT): go.mod go.sum $(wildcard ./pkg/**/*.go ./cmd/find_update/**/*.go)
	$(GO) build -tags $(GOTAGS) -ldflags $(GOBUILDLDFLAGS) $(GOBUILDFLAGS) -o $@ -v ./cmd/find_update

updater$(BINSUFFIX)$(BINEXT): go.mod go.sum $(wildcard ./pkg/**/*.go ./cmd/updater/**/*.go) build-fltk
	$(GO) build -tags $(GOTAGS) -ldflags $(GOBUILDLDFLAGS)\ $(GOBUILDGUILDFLAGS) $(GOBUILDFLAGS) $(GOBUILDGUIFLAGS) -o $@ -v ./cmd/updater

#
# PACKAGING TASKS
#

.PHONY: copy-libs-%

copy-libs-%: %
	$(MAKE) -s TARGET=$< copy-libs

.PHONY: copy-libs

# Extract list of shared library files that are used by imports, and copy them.
copy-libs: $(TARGET)
	$(OBJDUMP) -p $(TARGET) | \
		grep -E "NEEDED|DLL Name:" | \
		sed -e "s/\t*DLL Name: //g" -e "s/\s*NEEDED\s*//g"  | \
		grep -Evi $(DLL_BLACKLIST) | \
		tr "\n" " " | \
		xargs -r -t $(MAKE)

%: $(LIB_PREFIX)/%
	cp $< $@
	$(MAKE) -s TARGET=$@ copy-libs

.PHONY: clean-fltk

clean-fltk:
	$(RM) -r fltk_build

.PHONY: clean

clean: clean-fltk
	$(RM) $(addsuffix $(BINSUFFIX)$(BINEXT),$(BINS)) *$(DLLEXT) *$(DLLEXT).*
