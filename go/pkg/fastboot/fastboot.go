package fastboot

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
)

type responseType string

const (
	okay responseType = "OKAY"
	fail responseType = "FAIL"
	data responseType = "DATA"
	info responseType = "INFO"
	text responseType = "TEXT"
)

// FastBootError represents an error message returned by the FastBoot client
// device.
type FastBootError struct {
	Message string
}

// Error implements error.
func (f *FastBootError) Error() string {
	return fmt.Sprintf("fastboot request failed: %s", f.Message)
}

var _ error = (*FastBootError)(nil)

type FastBootChannel struct {
	infoC         <-chan string
	textC         <-chan string
	resultC       <-chan []byte
	readyForDataC <-chan uint32
	errorC        <-chan error

	logger *slog.Logger
	w      ContextWriter
}

var (
	ErrUnexpectedResponse = errors.New("unexpected response")
	ErrMaxLengthExceeded  = errors.New("max length exceeded")
)

type UnexpectedDataSizeError struct {
	Purpose                      string
	ActualLength, ExpectedLength uint64
}

// Error implements error.
func (e *UnexpectedDataSizeError) Error() string {
	return fmt.Sprintf("unexpected data size: expected %s size of %d but got %d instead",
		e.Purpose,
		e.ExpectedLength,
		e.ActualLength)
}

type TooShortPayloadError struct {
	Purpose                      string
	ActualLength, ExpectedLength uint64
}

// Error implements error.
func (e *TooShortPayloadError) Error() string {
	return fmt.Sprintf("too short payload: expected %s length of %d but got %d instead",
		e.Purpose,
		e.ExpectedLength,
		e.ActualLength)
}

var _ error = (*TooShortPayloadError)(nil)

func NewFastBootChannel(
	ctx context.Context,
	logger *slog.Logger,
	r ContextReader,
	w ContextWriter,
) *FastBootChannel {
	infoC := make(chan string, 1)
	textC := make(chan string, 1)
	resultC := make(chan []byte, 1)
	readyForDataC := make(chan uint32, 1)
	errorC := make(chan error, 1)

	fb := &FastBootChannel{
		infoC:         infoC,
		textC:         textC,
		resultC:       resultC,
		readyForDataC: readyForDataC,
		errorC:        errorC,
		w:             w,
		logger:        logger,
	}

	go func() {
		defer close(infoC)
		defer close(textC)
		defer close(resultC)
		defer close(errorC)
		defer close(readyForDataC)

		var textBuf *bytes.Buffer

		msgBuf := make([]byte, 512)
		for {
			n, err := r.ReadContext(ctx, msgBuf)
			if err != nil {
				errorC <- fmt.Errorf("read error: %w", err)
				return
			}
			if n < 4 {
				errorC <- &TooShortPayloadError{
					Purpose:        "message type",
					ExpectedLength: 4,
					ActualLength:   uint64(n),
				}
				return
			}
			logger.Debug("H<-C\n" + hex.Dump(msgBuf[0:n]))
			responseType := responseType(msgBuf[0:4])
			msgBuf = msgBuf[4:n]
			switch responseType {
			case okay:
				resultC <- msgBuf

			case fail:
				// rest of message provides text to present to the user
				errorC <- &FastBootError{
					Message: string(msgBuf),
				}

			case data: // ready for data, gives us uint32 size
				if len(msgBuf) < 8 {
					errorC <- &TooShortPayloadError{
						Purpose:        "allocated data length",
						ExpectedLength: 8,
						ActualLength:   uint64(len(msgBuf)),
					}
					return
				}
				dataLenBytes := make([]byte, 4)
				if _, err := hex.Decode(dataLenBytes, msgBuf[0:8]); err != nil {
					errorC <- err
				}
				readyForDataC <- binary.BigEndian.Uint32(dataLenBytes)

			case info: // informative message
				infoC <- string(msgBuf)

			case text: // arbitrary data, null-terminated
				for len(msgBuf) > 0 {
					copyLength := len(msgBuf)

					// copy everything up to null terminator if one exists
					nullIndex := bytes.IndexByte(msgBuf, 0)
					if nullIndex >= 0 {
						copyLength = nullIndex
					}
					textBuf.Write(msgBuf[0:copyLength])

					// leave rest in msgBuf
					msgBuf = msgBuf[copyLength:]

					// if there was a terminator, flush this payload
					if nullIndex >= 0 {
						textC <- textBuf.String()
						textBuf.Reset()

						// skip null terminator on next read
						msgBuf = msgBuf[1:]
					}
				}

			default:
				errorC <- ErrUnexpectedResponse
				return
			}
		}
	}()

	return fb
}

func (fb *FastBootChannel) InfoC() <-chan string {
	return fb.infoC
}

func (fb *FastBootChannel) TextC() <-chan string {
	return fb.textC
}

// Command executes an arbitrary formatted command.
//
// An error will be returned if a FAIL or any response type that is not OKAY,
// TEXT or INFO is transmitted back by the client.
func (fb *FastBootChannel) Command(ctx context.Context, cmd string, param ...interface{}) ([]byte, error) {
	msg := []byte(fmt.Sprintf(cmd, param...))
	fb.logger.Debug("H->C\n" + hex.Dump(msg))
	if _, err := fb.w.WriteContext(ctx, msg); err != nil {
		return nil, fmt.Errorf("write failed: %w", err)
	}
	select {
	case data := <-fb.resultC:
		return data, nil
	case err := <-fb.errorC:
		return nil, err
	}
}

func (fb *FastBootChannel) dataCommand(
	ctx context.Context,
	r io.Reader,
	size uint32,
	cmd string,
	param ...interface{},
) ([]byte, error) {
	msg := []byte(fmt.Sprintf(cmd, param...))
	fb.logger.Debug("H->C\n" + hex.Dump(msg))
	if _, err := fb.w.WriteContext(ctx, msg); err != nil {
		return nil, fmt.Errorf("write failed: %w", err)
	}
	// we expect DATA on success or FAIL on failure
	select {
	case data := <-fb.resultC: // okay
		return data, ErrUnexpectedResponse
	case err := <-fb.errorC: // error/fail
		return nil, err
	case returnedSize := <-fb.readyForDataC: // data
		if returnedSize != size {
			return nil, &UnexpectedDataSizeError{
				Purpose:        "allocated data buffer",
				ExpectedLength: uint64(size),
				ActualLength:   uint64(returnedSize),
			}
		}
	}

	// now we stream our data to the client
	buf := make([]byte, 128*1024 /*128k*/)
	for {
		n, err := r.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		fb.logger.Debug("H->C\n" + hex.Dump(buf[0:n]))
		_, err = fb.w.WriteContext(ctx, buf[0:n])
		if err != nil {
			return nil, err
		}
	}

	// and now we await the final response
	select {
	case result := <-fb.resultC:
		return result, nil
	case err := <-fb.errorC:
		return nil, err
	}
}

// GetVar requests a config/version variable's contents from the bootloader. If
// the variable is unknown, either a *FastBootError is returned or the contents
// will be empty, depending on implementation on the hardware side.
func (fb *FastBootChannel) GetVar(ctx context.Context, varName string) (string, error) {
	data, err := fb.Command(ctx, "getvar:%s", varName)
	var content string
	if data != nil {
		content = string(data)
	}
	return content, err
}

// Download writes data to the client device's memory to be later used by Boot,
// Flash, etc.
//
// The size to be transmitted is calculated from the offset at which r ends.
//
// The command will fail if there is not enough space in RAM or if the data size
// exceeds math.MaxUint32 in bytes.
func (fb *FastBootChannel) DownloadFromReadSeeker(ctx context.Context, r io.ReadSeeker) error {
	len, err := r.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	if len > math.MaxUint32 {
		return ErrMaxLengthExceeded
	}

	return fb.DownloadFromReader(ctx, r, uint32(len))
}

// DownloadFromReader writes data to the client device's memory to be later used
// by Boot, Flash, etc.
//
// The true size must to be passed as the size parameter, the client device's
// implementation needs the value for proper allocation on its side.
//
// The command will fail if there is not enough space in RAM.
func (fb *FastBootChannel) DownloadFromReader(ctx context.Context, r io.Reader, size uint32) error {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, size)

	_, err := fb.dataCommand(ctx, r, size, "download:%08x", size)
	return err
}

// Flash tells the device to write the previously downloaded image to the named
// partition (if possible).
func (fb *FastBootChannel) Flash(ctx context.Context, partition string) error {
	_, err := fb.Command(ctx, "flash:%s", partition)
	return err
}

// Erase tells the device to erase the indicated partition (clear to 0xFFs).
func (fb *FastBootChannel) Erase(ctx context.Context, partition string) error {
	_, err := fb.Command(ctx, "erase:%s", partition)
	return err
}

// Boot tells the device to boot into the previously downloaded boot.img.
func (fb *FastBootChannel) Boot(ctx context.Context) error {
	_, err := fb.Command(ctx, "boot")
	return err
}

// Continue tells the device to continue booting as normal (if possible).
func (fb *FastBootChannel) Continue(ctx context.Context) error {
	_, err := fb.Command(ctx, "continue")
	return err
}

// Reboot tells the device to reboot.
func (fb *FastBootChannel) Reboot(ctx context.Context) error {
	_, err := fb.Command(ctx, "reboot")
	return err
}

// Reboot tells the device to reboot back into the bootloader. Useful for
// upgrade processes that require upgrading the bootloader and then upgrading
// other partitions using the new bootloader.
func (fb *FastBootChannel) RebootBootloader(ctx context.Context) error {
	_, err := fb.Command(ctx, "reboot-bootloader")
	return err
}
