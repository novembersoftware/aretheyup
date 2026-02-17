package routes

import (
	"github.com/gin-gonic/gin"
)

type Service struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	URL           string `json:"url"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	Status        string `json:"status"`
	RecentReports int    `json:"recent_reports"`
}

func getIndex(c *gin.Context) {
	services := []Service{
		{ID: "1", Slug: "google", URL: "https://google.com", Name: "Google", Category: "Search", Status: "Operational", RecentReports: 0},
		{ID: "2", Slug: "facebook", URL: "https://facebook.com", Name: "Facebook", Category: "Social", Status: "Degraded", RecentReports: 50},
		{ID: "3", Slug: "twitter", URL: "https://twitter.com", Name: "Twitter", Category: "Social", Status: "Outage", RecentReports: 200},
		{ID: "4", Slug: "aws", URL: "https://aws.amazon.com", Name: "AWS", Category: "Cloud", Status: "Outage", RecentReports: 100},
		{ID: "5", Slug: "github", URL: "https://github.com", Name: "GitHub", Category: "Developer", Status: "Operational", RecentReports: 15},
		{ID: "6", Slug: "discord", URL: "https://discord.com", Name: "Discord", Category: "Communication", Status: "Operational", RecentReports: 2},
		{ID: "7", Slug: "instagram", URL: "https://instagram.com", Name: "Instagram", Category: "Social", Status: "Operational", RecentReports: 0},
		{ID: "8", Slug: "tiktok", URL: "https://tiktok.com", Name: "TikTok", Category: "Social", Status: "Outage", RecentReports: 100},
		{ID: "9", Slug: "youtube", URL: "https://youtube.com", Name: "YouTube", Category: "Social", Status: "Operational", RecentReports: 100},
		{ID: "10", Slug: "reddit", URL: "https://reddit.com", Name: "Reddit", Category: "Social", Status: "Operational", RecentReports: 100},
		{ID: "11", Slug: "linkedin", URL: "https://linkedin.com", Name: "LinkedIn", Category: "Social", Status: "Operational", RecentReports: 100},
		{ID: "12", Slug: "pinterest", URL: "https://pinterest.com", Name: "Pinterest", Category: "Social", Status: "Operational", RecentReports: 100},
		{ID: "13", Slug: "snapchat", URL: "https://snapchat.com", Name: "Snapchat", Category: "Social", Status: "Operational", RecentReports: 100},
		{ID: "14", Slug: "whatsapp", URL: "https://whatsapp.com", Name: "WhatsApp", Category: "Communication", Status: "Operational", RecentReports: 100},
		{ID: "15", Slug: "telegram", URL: "https://telegram.org", Name: "Telegram", Category: "Communication", Status: "Operational", RecentReports: 100},
		{ID: "16", Slug: "skype", URL: "https://skype.com", Name: "Skype", Category: "Communication", Status: "Operational", RecentReports: 100},
		{ID: "17", Slug: "viber", URL: "https://viber.com", Name: "Viber", Category: "Communication", Status: "Operational", RecentReports: 100},
		{ID: "18", Slug: "whatsapp", URL: "https://whatsapp.com", Name: "WhatsApp", Category: "Communication", Status: "Operational", RecentReports: 100},
		{ID: "19", Slug: "telegram", URL: "https://telegram.org", Name: "Telegram", Category: "Communication", Status: "Operational", RecentReports: 100},
		{ID: "20", Slug: "skype", URL: "https://skype.com", Name: "Skype", Category: "Communication", Status: "Operational", RecentReports: 100},
	}

	c.HTML(200, "index.html", gin.H{
		"Services": services,
	})
}
