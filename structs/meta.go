package structs

type Meta struct {
	Title         string
	Description   string
	Keywords      []string
	CanonicalURL  string
	Robots        string
	ThemeColor    string
	Author        string
	SiteName      string
	Locale        string
	PublishedTime string
	ModifiedTime  string
	ImageURL      string
	ImageAlt      string

	OpenGraph OpenGraphMeta
	Twitter   TwitterMeta
	JSONLD    []string

	PrevURL string
	NextURL string
}

type MetaInput struct {
	Title         string
	Description   string
	Keywords      []string
	CanonicalPath string
	Robots        string
	ThemeColor    string
	Author        string
	SiteName      string
	Locale        string
	PublishedTime string
	ModifiedTime  string
	ImageURL      string
	ImageAlt      string
	ServiceName   string
	ServiceSlug   string
	Status        string
	JSONLD        []string
	PrevURL       string
	NextURL       string
}

type OpenGraphMeta struct {
	Title       string
	Description string
	Type        string
	URL         string
	SiteName    string
	Locale      string
	ImageURL    string
	ImageAlt    string
	ImageWidth  int
	ImageHeight int
}

type TwitterMeta struct {
	Card        string
	Site        string
	Creator     string
	Title       string
	Description string
	ImageURL    string
	ImageAlt    string
}
