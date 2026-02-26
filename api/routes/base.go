package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
	"github.com/novembersoftware/aretheyup/utils"
)

// GET /
func getIndexPage(c *gin.Context) {
	c.HTML(200, "index.html", gin.H{
		"Meta": utils.BuildMeta(c, structs.MetaInput{
			Title:         "Are they up? | Live Service Status & Outage Reports",
			Description:   "Monitor live user-reported outages and service reliability in real time.",
			CanonicalPath: "/",
			Keywords:      []string{"service status", "outage monitor", "is it down", "downtime reports"},
			ImageURL:      "/og-image.png",
			ImageAlt:      "Are they up? service status monitor",
		}),
	})
}

// GET /:slug
func getServicePage(c *gin.Context, store *storage.Storage) {
	slug := c.Param("slug")
	service, err := store.GetServiceBySlug(c.Request.Context(), slug)
	if err != nil {
		c.HTML(404, "not-found.html", gin.H{
			"Meta": utils.BuildMeta(c, structs.MetaInput{
				Title:         "Page Not Found | Are they up?",
				Description:   "The page you requested could not be found.",
				CanonicalPath: c.Request.URL.Path,
				Robots:        "noindex,follow",
				ImageURL:      "/og-image.png",
				ImageAlt:      "Are they up? service status monitor",
			}),
		})
		return
	}
	c.HTML(200, "service.html", gin.H{
		"Slug": slug,
		"Meta": utils.BuildMeta(c, structs.MetaInput{
			ServiceName:   service.Name,
			ServiceSlug:   service.Slug,
			Description:   service.Description,
			CanonicalPath: "/" + service.Slug,
			Keywords:      []string{service.Name + " status", "is " + service.Name + " down", service.Name + " outage"},
			ImageURL:      "/og-image.png",
			ImageAlt:      service.Name + " status page",
			ModifiedTime:  service.UpdatedAt.UTC().Format(time.RFC3339),
		}),
	})
}
