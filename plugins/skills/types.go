package skills

// Skill is a local DTO representing a user skill. It mirrors the fields from
// modules/skillstore.UserSkill that this plugin actually uses, so the plugin
// does not import the skillstore module directly.
type Skill struct {
	Name         string
	Description  string
	Instructions string
	Category     string
	Icon         string
	IsCustom     bool
}
