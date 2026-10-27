package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousEnabled := common.RootRedirectEnabled
	previousURL := common.RootRedirectURL
	t.Cleanup(func() {
		common.RootRedirectEnabled = previousEnabled
		common.RootRedirectURL = previousURL
	})

	newRouter := func() *gin.Engine {
		r := gin.New()
		r.Use(RootRedirect())
		r.NoRoute(func(c *gin.Context) {
			c.String(http.StatusOK, "home")
		})
		return r
	}

	t.Run("disabled keeps home", func(t *testing.T) {
		common.RootRedirectEnabled = false
		common.RootRedirectURL = "https://basamp.com/"
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		newRouter().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "home", w.Body.String())
	})

	t.Run("enabled redirects exact root", func(t *testing.T) {
		common.RootRedirectEnabled = true
		common.RootRedirectURL = "https://basamp.com/"
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		newRouter().ServeHTTP(w, req)
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "https://basamp.com/", w.Header().Get("Location"))
	})

	t.Run("path with suffix is not redirected", func(t *testing.T) {
		common.RootRedirectEnabled = true
		common.RootRedirectURL = "https://basamp.com/"
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/sign-in?redirect=%2Fdashboard", nil)
		newRouter().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "home", w.Body.String())
	})

	t.Run("enabled with invalid url falls through", func(t *testing.T) {
		common.RootRedirectEnabled = true
		common.RootRedirectURL = "/relative"
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		newRouter().ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "home", w.Body.String())
	})
}
