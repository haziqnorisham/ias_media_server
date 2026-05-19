package onvif

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gov "github.com/gowvp/onvif"
	gov_device "github.com/gowvp/onvif/device"
	"github.com/gowvp/onvif/media"
	xsd_onvif "github.com/gowvp/onvif/xsd/onvif"
	sdk_device "github.com/gowvp/onvif/sdk/device"
	sdk_media "github.com/gowvp/onvif/sdk/media"
)

type DeviceInfo struct {
	Manufacturer    string `json:"manufacturer"`
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmware_version"`
	SerialNumber    string `json:"serial_number"`
	HardwareID      string `json:"hardware_id"`
}

type ProfileInfo struct {
	Token       string `json:"token"`
	Name        string `json:"name"`
	Encoding    string `json:"encoding"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	RTSPURL     string `json:"rtsp_url"`
	SnapshotURL string `json:"snapshot_url"`
}

type Client struct {
	dev    *gov.Device
	params gov.DeviceParams
}

func Connect(ip string, port int, username, password string) (*Client, error) {
	params := gov.DeviceParams{
		Xaddr:      fmt.Sprintf("%s:%d", ip, port),
		Username:   username,
		Password:   password,
		HttpClient: &http.Client{Timeout: 10 * time.Second},
	}
	dev, err := gov.NewDevice(params)
	if err != nil {
		return nil, fmt.Errorf("onvif connect: %w", err)
	}
	return &Client{dev: dev, params: params}, nil
}

func (c *Client) GetDeviceInfo() (*DeviceInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := sdk_device.Call_GetDeviceInformation(ctx, c.dev, gov_device.GetDeviceInformation{})
	if err != nil {
		return nil, fmt.Errorf("GetDeviceInformation: %w", err)
	}
	return &DeviceInfo{
		Manufacturer:    string(resp.Manufacturer),
		Model:           string(resp.Model),
		FirmwareVersion: string(resp.FirmwareVersion),
		SerialNumber:    string(resp.SerialNumber),
		HardwareID:      string(resp.HardwareId),
	}, nil
}

func (c *Client) GetProfiles() ([]ProfileInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := sdk_media.Call_GetProfiles(ctx, c.dev, media.GetProfiles{})
	if err != nil {
		return nil, fmt.Errorf("GetProfiles: %w", err)
	}

	profiles := make([]ProfileInfo, 0, len(resp.Profiles))
	for _, p := range resp.Profiles {
		pi := ProfileInfo{
			Token:  string(p.Token),
			Name:   string(p.Name),
			Width:  int(p.VideoEncoderConfiguration.Resolution.Width),
			Height: int(p.VideoEncoderConfiguration.Resolution.Height),
		}
		if enc := p.VideoEncoderConfiguration.Encoding; enc != "" {
			pi.Encoding = string(enc)
		}

		uriResp, err := sdk_media.Call_GetStreamUri(ctx, c.dev, media.GetStreamUri{
			StreamSetup: xsd_onvif.StreamSetup{
				Stream:    "RTP-Unicast",
				Transport: xsd_onvif.Transport{Protocol: "RTSP"},
			},
			ProfileToken: p.Token,
		})
		if err == nil {
			pi.RTSPURL = string(uriResp.MediaUri.Uri)
		}

		snapResp, err := sdk_media.Call_GetSnapshotUri(ctx, c.dev, media.GetSnapshotUri{
			ProfileToken: p.Token,
		})
		if err == nil {
			pi.SnapshotURL = string(snapResp.MediaUri.Uri)
		}

		profiles = append(profiles, pi)
	}
	return profiles, nil
}
