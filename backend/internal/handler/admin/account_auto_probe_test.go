package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupAccountAutoProbeRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewAccountHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler.SetAccountAutoProbeService(service.NewAccountAutoProbeService(nil, nil, nil, nil))

	router := gin.New()
	router.GET("/admin/accounts/auto-probe/settings", handler.GetAccountAutoProbeSettings)
	router.PUT("/admin/accounts/auto-probe/settings", handler.UpdateAccountAutoProbeSettings)
	return router
}

func TestAccountHandlerGetAccountAutoProbeSettingsReturnsOptInDefaults(t *testing.T) {
	router := setupAccountAutoProbeRouter()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/accounts/auto-probe/settings", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data service.AccountAutoProbeSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Data.Enabled)
	require.Equal(t, 30, response.Data.IntervalMinutes)
	require.True(t, response.Data.AutoRecover)
}

func TestAccountHandlerUpdateAccountAutoProbeSettingsUnavailableWithoutSettingService(t *testing.T) {
	router := setupAccountAutoProbeRouter()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/admin/accounts/auto-probe/settings",
		bytes.NewBufferString(`{"enabled":true,"interval_minutes":5,"auto_recover":true}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}
