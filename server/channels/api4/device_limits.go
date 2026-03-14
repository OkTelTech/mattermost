// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

type deviceSession struct {
	*model.Session
	DeviceType string `json:"device_type"`
	IsCurrent  bool   `json:"is_current"`
}

type deviceLimitsPayload struct {
	MaxMobileDevices  int  `json:"max_mobile_devices"`
	MaxDesktopDevices int  `json:"max_desktop_devices"`
	BypassLimit       bool `json:"bypass_limit,omitempty"`
}

func getUserDeviceSessions(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	sessions, appErr := c.App.GetSessions(c.AppContext, c.Params.UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	currentSessionID := c.AppContext.Session().Id
	deviceSessions := make([]*deviceSession, 0, len(sessions))
	for _, s := range sessions {
		isCurrent := s.Id == currentSessionID
		s.Sanitize()
		dt := "desktop"
		if s.IsMobileApp() {
			dt = "mobile"
		}
		deviceSessions = append(deviceSessions, &deviceSession{Session: s, DeviceType: dt, IsCurrent: isCurrent})
	}

	js, err := json.Marshal(deviceSessions)
	if err != nil {
		c.Err = model.NewAppError("getUserDeviceSessions", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
		return
	}

	if _, err := w.Write(js); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getUserDeviceLimits(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	user, appErr := c.App.GetUser(c.Params.UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	payload := deviceLimitsPayload{
		MaxMobileDevices:  user.GetMaxMobileDevices(),
		MaxDesktopDevices: user.GetMaxDesktopDevices(),
		BypassLimit:       user.IsSystemAdmin(),
	}

	js, err := json.Marshal(payload)
	if err != nil {
		c.Err = model.NewAppError("getUserDeviceLimits", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
		return
	}

	if _, err := w.Write(js); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func updateUserDeviceLimits(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	var payload deviceLimitsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		c.SetInvalidParamWithErr("body", err)
		return
	}

	if payload.MaxMobileDevices < 1 || payload.MaxDesktopDevices < 1 {
		c.SetInvalidParam("max_mobile_devices or max_desktop_devices must be >= 1")
		return
	}

	user, appErr := c.App.UpdateUserDeviceLimits(c.AppContext, c.Params.UserId, payload.MaxMobileDevices, payload.MaxDesktopDevices)
	if appErr != nil {
		c.Err = appErr
		return
	}

	js, err := json.Marshal(deviceLimitsPayload{
		MaxMobileDevices:  user.GetMaxMobileDevices(),
		MaxDesktopDevices: user.GetMaxDesktopDevices(),
	})
	if err != nil {
		c.Err = model.NewAppError("updateUserDeviceLimits", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
		return
	}

	if _, err := w.Write(js); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
