package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"go_onvif/db"
	"go_onvif/onvif"
	"go_onvif/stream"
)

type DeviceHandler struct {
	DB        *sql.DB
	StreamMgr *stream.StreamManager
}

type createDeviceRequest struct {
	Name     string `json:"name"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type updateDeviceRequest struct {
	Name               string `json:"name"`
	IP                 string `json:"ip"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	StreamProfileToken string `json:"stream_profile_token"`
	Notes              string `json:"notes"`
}

type setStreamProfileRequest struct {
	Token string `json:"token"`
}

func (h *DeviceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Name == "" || req.IP == "" || req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, ip, username, and password are required"})
		return
	}
	if req.Port == 0 {
		req.Port = 80
	}

	device, err := db.InsertDevice(h.DB, req.Name, req.IP, req.Port, req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, device)
}

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	devices, err := db.ListDevices(h.DB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (h *DeviceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
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
	writeJSON(w, http.StatusOK, device)
}

func (h *DeviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		return
	}

	existing, err := db.GetDeviceByID(h.DB, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}

	var req updateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	name := or(req.Name, existing.Name)
	ip := or(req.IP, existing.IP)
	port := existing.Port
	if req.Port != 0 {
		port = req.Port
	}
	username := or(req.Username, existing.Username)
	password := or(req.Password, existing.Password)
	streamProfileToken := or(req.StreamProfileToken, existing.StreamProfileToken)
	notes := or(req.Notes, existing.Notes)

	device, err := db.UpdateDevice(h.DB, id, name, ip, port, username, password, streamProfileToken, notes)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, device)
}

func (h *DeviceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		return
	}

	deleted, err := db.DeleteDevice(h.DB, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *DeviceHandler) Profiles(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
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

	client, err := onvif.Connect(device.IP, device.Port, device.Username, device.Password)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "onvif connect: " + err.Error()})
		return
	}

	profiles, err := client.GetProfiles()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "onvif get profiles: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"device_id": id,
		"profiles":  profiles,
	})
}

func (h *DeviceHandler) SetStreamProfile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		return
	}

	var req setStreamProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token is required"})
		return
	}

	if err := db.SetStreamProfile(h.DB, id, req.Token); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	device, _ := db.GetDeviceByID(h.DB, id)
	if device == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "device not found after update"})
		return
	}

	h.StreamMgr.StopStream(id)

	client, err := onvif.Connect(device.IP, device.Port, device.Username, device.Password)
	if err != nil {
		log.Printf("stream-profile restart %d: onvif connect: %v", id, err)
		writeJSON(w, http.StatusOK, device)
		return
	}
	rtspURL, err := client.GetStreamUri(req.Token)
	if err != nil {
		log.Printf("stream-profile restart %d: GetStreamUri: %v", id, err)
		writeJSON(w, http.StatusOK, device)
		return
	}

	h.StreamMgr.StartStream(id, device.Name, req.Token, rtspURL)
	writeJSON(w, http.StatusOK, device)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func or(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}
