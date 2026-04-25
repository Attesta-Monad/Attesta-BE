package model

type Period struct {
	FirstCommit string `json:"first_commit"`
	LastCommit  string `json:"last_commit"`
}

type SkillProof struct {
	PrimaryLanguage     string   `json:"primary_language"`
	SecondaryLanguages  []string `json:"secondary_languages"`
	ContributionTypes   []string `json:"contribution_types"`
	I18nOnly            bool     `json:"i18n_only"`
	SkillTags           []string `json:"skill_tags"`
	ContributionQuality string   `json:"contribution_quality"`
	ConfidenceScore     int      `json:"confidence_score"`
	RedFlags            []string `json:"red_flags"`
	Summary             string   `json:"summary"`
	Period              Period   `json:"period"`
}
