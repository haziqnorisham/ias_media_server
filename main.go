package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go_onvif/db"

	onvif "github.com/gowvp/onvif"
	"github.com/gowvp/onvif/device"
	"github.com/gowvp/onvif/media"
	xsd_onvif "github.com/gowvp/onvif/xsd/onvif"
	sdk_device "github.com/gowvp/onvif/sdk/device"
	sdk_media "github.com/gowvp/onvif/sdk/media"
)

func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== ONVIF Camera Tester ===")

	sqlite, err := db.Init("onvif.db")
	if err != nil {
		fmt.Println("Database init failed:", err)
		os.Exit(1)
	}
	defer sqlite.Close()
	fmt.Println("Database: OK")

	ip := prompt(reader, "Camera IP: ")
	username := prompt(reader, "Username: ")
	password := prompt(reader, "Password: ")

	ctx := context.Background()

	dev, err := onvif.NewDevice(onvif.DeviceParams{
		Xaddr:    ip + ":80",
		Username: username,
		Password: password,
	})
	if err != nil {
		fmt.Println("Failed to connect to device:", err)
		os.Exit(1)
	}

	fmt.Println("\n--- Device Information ---")
	devInfo, err := sdk_device.Call_GetDeviceInformation(ctx, dev, device.GetDeviceInformation{})
	if err != nil {
		fmt.Println("  Error:", err)
	} else {
		fmt.Printf("  Manufacturer:   %s\n", devInfo.Manufacturer)
		fmt.Printf("  Model:          %s\n", devInfo.Model)
		fmt.Printf("  Firmware:       %s\n", devInfo.FirmwareVersion)
		fmt.Printf("  Serial Number:  %s\n", devInfo.SerialNumber)
		fmt.Printf("  Hardware ID:    %s\n", devInfo.HardwareId)
	}

	fmt.Println("\n--- Capabilities ---")
	caps, err := sdk_device.Call_GetCapabilities(ctx, dev, device.GetCapabilities{Category: "All"})
	if err != nil {
		fmt.Println("  Error:", err)
	} else {
		c := caps.Capabilities
		if c.Analytics.XAddr != "" {
			fmt.Printf("  Analytics:  %s\n", c.Analytics.XAddr)
		}
		if c.Device.XAddr != "" {
			fmt.Printf("  Device:     %s\n", c.Device.XAddr)
		}
		if c.Events.XAddr != "" {
			fmt.Printf("  Events:     %s\n", c.Events.XAddr)
		}
		if c.Imaging.XAddr != "" {
			fmt.Printf("  Imaging:    %s\n", c.Imaging.XAddr)
		}
		if c.Media.XAddr != "" {
			fmt.Printf("  Media:      %s\n", c.Media.XAddr)
		}
		if c.PTZ.XAddr != "" {
			fmt.Printf("  PTZ:        %s\n", c.PTZ.XAddr)
		}
	}

	fmt.Println("\n--- System Date & Time ---")
	sysTime, err := sdk_device.Call_GetSystemDateAndTime(ctx, dev, device.GetSystemDateAndTime{})
	if err != nil {
		fmt.Println("  Error:", err)
	} else {
		fmt.Printf("  UTC DateTime:   %v\n", sysTime.SystemDateAndTime.UTCDateTime)
		fmt.Printf("  Local DateTime: %v\n", sysTime.SystemDateAndTime.LocalDateTime)
		fmt.Printf("  Time Zone:      %s\n", sysTime.SystemDateAndTime.TimeZone.TZ)
	}

	fmt.Println("\n--- Network Interfaces ---")
	netResp, err := sdk_device.Call_GetNetworkInterfaces(ctx, dev, device.GetNetworkInterfaces{})
	if err != nil {
		fmt.Println("  Error:", err)
	} else {
		ni := netResp.NetworkInterfaces
		fmt.Printf("  Name:    %s\n", ni.Info.Name)
		if ni.Info.HwAddress != "" {
			fmt.Printf("  MAC:     %s\n", ni.Info.HwAddress)
		}
		fmt.Printf("  Enabled: %v\n", ni.Enabled)
		fmt.Printf("  MTU:     %d\n", ni.Info.MTU)
	}

	fmt.Println("\n--- Hostname ---")
	host, err := sdk_device.Call_GetHostname(ctx, dev, device.GetHostname{})
	if err != nil {
		fmt.Println("  Error:", err)
	} else {
		fmt.Printf("  Name:      %s\n", host.HostnameInformation.Name)
		fmt.Printf("  From DHCP: %v\n", host.HostnameInformation.FromDHCP)
	}

	fmt.Println("\n--- Scopes ---")
	scopes, err := sdk_device.Call_GetScopes(ctx, dev, device.GetScopes{})
	if err != nil {
		fmt.Println("  Error:", err)
	} else {
		fmt.Printf("  Scope: %s\n", scopes.Scopes.ScopeItem)
	}

	fmt.Println("\n--- Profiles ---")
	profiles, err := sdk_media.Call_GetProfiles(ctx, dev, media.GetProfiles{})
	if err != nil {
		fmt.Println("  Error:", err)
	} else {
		var primarySnapshotURL string

		for _, p := range profiles.Profiles {
			fmt.Printf("  Name:        %s\n", p.Name)
			fmt.Printf("  Token:       %s\n", p.Token)
			fmt.Printf("  Fixed:       %v\n", p.Fixed)
			fmt.Printf("  Encoding:    %s\n", p.VideoEncoderConfiguration.Encoding)
			fmt.Printf("  Resolution:  %dx%d\n",
				p.VideoEncoderConfiguration.Resolution.Width,
				p.VideoEncoderConfiguration.Resolution.Height)

			uri, err := sdk_media.Call_GetStreamUri(ctx, dev, media.GetStreamUri{
				StreamSetup: xsd_onvif.StreamSetup{
					Stream:    "RTP-Unicast",
					Transport: xsd_onvif.Transport{Protocol: "RTSP"},
				},
				ProfileToken: p.Token,
			})
			if err == nil {
				fmt.Printf("  RTSP URL:    %s\n", uri.MediaUri.Uri)
			}

			snap, err := sdk_media.Call_GetSnapshotUri(ctx, dev, media.GetSnapshotUri{
				ProfileToken: p.Token,
			})
			if err == nil && string(snap.MediaUri.Uri) != "" {
				fmt.Printf("  Snapshot URL: %s\n", snap.MediaUri.Uri)
				if primarySnapshotURL == "" {
					primarySnapshotURL = string(snap.MediaUri.Uri)
				}
			}

			fmt.Println()
		}

		if primarySnapshotURL != "" {
			fmt.Println("--- Capturing Snapshot ---")
			saveSnapshot(primarySnapshotURL, username, password)
		}
	}

	fmt.Println("--- Done ---")
}

func saveSnapshot(url, username, password string) {
	snapshotDir := "snapshots"
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		fmt.Printf("  Failed to create snapshots directory: %v\n", err)
		return
	}

	timestamp := time.Now().Format("2006-01-02_150405")
	filename := filepath.Join(snapshotDir, "snapshot_"+timestamp+".jpg")

	fmt.Printf("  Downloading snapshot from: %s\n", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("  Failed to create request: %v\n", err)
		return
	}
	req.SetBasicAuth(username, password)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  Failed to download snapshot: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  Snapshot request returned status: %s\n", resp.Status)
		return
	}

	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("  Failed to create file: %v\n", err)
		return
	}
	defer file.Close()

	written, err := io.Copy(file, resp.Body)
	if err != nil {
		fmt.Printf("  Failed to save snapshot: %v\n", err)
		return
	}

	fmt.Printf("  Snapshot saved: %s (%d bytes)\n", filename, written)
}
