package handler

import (
	"database/sql"
	"net/http"
	"strconv"

	"go_onvif/db"
	"go_onvif/onvif"
	"go_onvif/stream"
)

type StreamHandler struct {
	Manager *stream.StreamManager
	DB      *sql.DB
}

func (h *StreamHandler) List(w http.ResponseWriter, r *http.Request) {
	sessions := h.Manager.ListSessions()
	if sessions == nil {
		sessions = []stream.StreamSession{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *StreamHandler) Start(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("device_id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		return
	}

	device, err := db.GetDeviceByID(h.DB, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if device == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}
	if device.StreamProfileToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no stream profile set for this device"})
		return
	}

	client, err := onvif.Connect(device.IP, device.Port, device.Username, device.Password)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "onvif connect: " + err.Error()})
		return
	}

	rtspURL, err := client.GetStreamUri(device.StreamProfileToken)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "onvif GetStreamUri: " + err.Error()})
		return
	}

	h.Manager.StartStream(device.ID, device.Name, device.StreamProfileToken, rtspURL)
	session := h.Manager.GetSession(device.ID)
	writeJSON(w, http.StatusCreated, session)
}

func (h *StreamHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("device_id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		return
	}

	h.Manager.StopStream(id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}
