package routes

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
	"github.com/novembersoftware/aretheyup/utils"
	"github.com/rs/zerolog/log"
)

type submitServiceRequest struct {
	Name        string `form:"name" json:"name"`
	Category    string `form:"category" json:"category"`
	HomepageURL string `form:"homepage_url" json:"homepage_url"`
	Email       string `form:"email" json:"email"`
	Company     string `form:"company" json:"company"`
}

// GET /submit-service
func getSubmitServicePage(c *gin.Context) {
	c.HTML(http.StatusOK, "submit-service.html", submitServiceFormData(c, submitServiceRequest{}, ""))
}

// POST /api/services/submit
func submitService(c *gin.Context, store *storage.Storage) {
	var req submitServiceRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.Respond(c, http.StatusBadRequest, "submit-service-form", submitServiceFormData(c, req, "Invalid submission payload."))
		return
	}

	if strings.TrimSpace(req.Company) != "" {
		utils.Respond(c, http.StatusAccepted, "submit-service-success", gin.H{})
		return
	}

	result, err := store.SubmitService(c.Request.Context(), storage.SubmitServiceInput{
		Name:                 req.Name,
		Category:             req.Category,
		HomepageURL:          req.HomepageURL,
		SubmitterEmail:       req.Email,
		SubmitterFingerprint: utils.RequestFingerprint(c),
	})
	if err != nil {
		var submitErr *storage.SubmitServiceError
		if errors.As(err, &submitErr) {
			handleSubmitServiceError(c, req, submitErr)
			return
		}

		log.Error().Err(err).Str("path", c.FullPath()).Msg("Service submission failed")
		utils.Respond(c, http.StatusInternalServerError, "error", gin.H{"error": "Failed to submit service"})
		return
	}

	utils.Respond(c, http.StatusCreated, "submit-service-success", gin.H{
		"service_slug": result.Service.Slug,
	})
}

func handleSubmitServiceError(c *gin.Context, req submitServiceRequest, submitErr *storage.SubmitServiceError) {
	if submitErr == nil {
		utils.Respond(c, http.StatusBadRequest, "submit-service-form", submitServiceFormData(c, req, "Unable to submit service."))
		return
	}

	if submitErr.Code == storage.SubmitServiceErrorDuplicate && submitErr.ExistingServiceSlug != "" && !requestWantsJSON(c) {
		utils.Respond(c, http.StatusOK, "submit-service-success", gin.H{
			"already_tracked": true,
			"service_slug":    submitErr.ExistingServiceSlug,
		})
		return
	}

	status := http.StatusBadRequest
	if submitErr.Code == storage.SubmitServiceErrorDuplicate {
		status = http.StatusConflict
	}

	if requestWantsJSON(c) {
		response := gin.H{"error": submitErr.Message}
		if submitErr.ExistingServiceSlug != "" {
			response["service_slug"] = submitErr.ExistingServiceSlug
		}
		utils.Respond(c, status, "error", response)
		return
	}

	utils.Respond(c, status, "submit-service-form", submitServiceFormData(c, req, submitErr.Message))
}

func submitServiceFormData(c *gin.Context, req submitServiceRequest, errorMessage string) gin.H {
	category := strings.ToLower(strings.TrimSpace(req.Category))
	if category == "" {
		category = "other"
	}

	return gin.H{
		"Meta": utils.BuildMeta(c, structs.MetaInput{
			Title:         "Submit a service | Are they up?",
			Description:   "Add a service to the tracker in a few seconds.",
			CanonicalPath: "/submit-service",
			Robots:        "index,follow",
			Keywords:      []string{"submit service", "service status", "outage tracker"},
			ImageURL:      "/og-image.png",
			ImageAlt:      "Submit service on Are they up?",
		}),
		"name":         strings.TrimSpace(req.Name),
		"category":     category,
		"homepage_url": strings.TrimSpace(req.HomepageURL),
		"email":        strings.TrimSpace(req.Email),
		"categories":   storage.AllowedServiceCategories(),
		"error":        errorMessage,
	}
}

func requestWantsJSON(c *gin.Context) bool {
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "application/json")
}
