package main

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/PuerkitoBio/goquery"
)

const (
	updateURL     = "https://autoupdate.airmusictech.com/PrimeUpdates.xml"
	updateSiteURL = "https://enginedj.com/downloads"
)

type Reference struct {
	URL string `xml:"url,attr"`
}

type Version struct {
	Platform string     `xml:"platform,attr"`
	Number   string     `xml:"number,attr"`
	Summary  string     `xml:"summary"`
	PageURL  *Reference `xml:"page,omitempty"`
	ImageURL *Reference `xml:"image,omitempty"`
}

type Channel struct {
	Versions []Version `xml:"version"`
}

type Application struct {
	Id     string  `xml:"id,attr"`
	Newest Channel `xml:"newest"`
}

type UpdateInfo struct {
	Applications []Application `xml:"application"`
}

type DeviceEntry struct {
	FriendlyVendorName string
	FriendlyDeviceName string
	DeviceID           string
	ApplicationName    string
	ImageURL           string
	WindowsUpdaterURL  string
}

type Devices struct {
	Entries []DeviceEntry
}

func DecodeDevices(r io.Reader) (*Devices, error) {
	csvReader := csv.NewReader(r)
	csvReader.FieldsPerRecord = 6
	csvReader.Comma = ' '
	d := &Devices{
		Entries: []DeviceEntry{},
	}
	for {
		record, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		d.Entries = append(d.Entries, DeviceEntry{
			FriendlyVendorName: record[0],
			FriendlyDeviceName: record[1],
			DeviceID:           record[2],
			ApplicationName:    record[3],
			ImageURL:           record[4],
			WindowsUpdaterURL:  record[5],
		})
	}
	return d, nil
}

func EncodeDevices(w io.Writer, d *Devices) error {
	csvWriter := csv.NewWriter(w)
	csvWriter.UseCRLF = false
	csvWriter.Comma = ' '
	for _, entry := range d.Entries {
		err := csvWriter.Write([]string{
			entry.FriendlyVendorName,
			entry.FriendlyDeviceName,
			entry.DeviceID,
			entry.ApplicationName,
			entry.ImageURL,
			entry.WindowsUpdaterURL,
		})
		if err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return nil
}

type EngineOsReleaseDetails struct {
	MacMd5 string
	MacUrl string
	UsbMd5 string
	UsbUrl string
	WinMd5 string
	WinUrl string
}

type DownloadsWebsite struct {
	Props struct {
		PageProps struct {
			Page struct {
				Sections []struct {
					TypeName string `json:"__typename"`
					// EngineDesktopReleasesCollection
					EngineOsReleasesCollection struct {
						Items []struct {
							TypeName                    string `json:"__typename"`
							HardwareUnitLinksCollection struct {
								Items []struct {
									HardwareUnit struct {
										Title string
									}
									EngineOsReleaseDetails
								}
							}
						}
					}
				}
			}
		}
	}
}

func getUpdaterEXEs() (map[string]EngineOsReleaseDetails, error) {
	doc, err := goquery.NewDocument(updateSiteURL)
	if err != nil {
		return nil, err
	}

	str := doc.Find("script#__NEXT_DATA__").Text()
	var data DownloadsWebsite
	err = json.Unmarshal([]byte(str), &data)
	if err != nil {
		return nil, err
	}

	// b, _ := json.MarshalIndent(data, "", "  ")
	// os.Stdout.Write(b)

	result := map[string]EngineOsReleaseDetails{}

	for _, section := range data.Props.PageProps.Page.Sections {
		if section.TypeName != "PageSectionReleaseNotes" {
			continue
		}

		for _, release := range section.EngineOsReleasesCollection.Items {
			if release.TypeName != "DownloadsEngineOsRelease" {
				continue
			}

			for _, hardwareUnitLinks := range release.HardwareUnitLinksCollection.Items {
				result[hardwareUnitLinks.HardwareUnit.Title] = hardwareUnitLinks.EngineOsReleaseDetails
			}

			break // do not process any older releases
		}
		break
	}

	return result, nil
}

func main() {
	resp, err := http.Get(updateURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	autoUpdateInfo := new(UpdateInfo)
	err = xml.NewDecoder(resp.Body).Decode(autoUpdateInfo)
	if err != nil {
		log.Fatal(err)
	}

	updateInfo, err := getUpdaterEXEs()
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.OpenFile("devices.txt", os.O_RDWR, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	devices, err := DecodeDevices(f)
	if err != nil {
		log.Fatal(err)
	}

	for deviceIndex, device := range devices.Entries {
		log.Println("Expect", device.ApplicationName)
		for _, app := range autoUpdateInfo.Applications {
			log.Println("  Compare", app.Id)
			if app.Id == device.ApplicationName {

				// Update image URL
				log.Println("  Match, update image URL")
				imageURL := app.Newest.Versions[0].ImageURL.URL
				device.ImageURL = imageURL

				// Find Windows updater URL
				for _, hardwareUnitRelease := range updateInfo {
					if hardwareUnitRelease.UsbUrl == imageURL {
						log.Println("  Match, update Windows updater URL")
						device.WindowsUpdaterURL = hardwareUnitRelease.WinUrl
						break
					}
				}

				break
			}
		}

		devices.Entries[deviceIndex] = device
	}

	_, err = f.Seek(0, os.SEEK_SET)
	if err != nil {
		log.Fatal(err)
	}

	err = f.Truncate(0)
	if err != nil {
		log.Fatal(err)
	}

	err = EncodeDevices(f, devices)
	if err != nil {
		log.Fatal(err)
	}
}
