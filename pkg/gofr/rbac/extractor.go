package rbac

func RoleExtractor(config *Config) (roles []string, err error) {
	if config.RoleExtractorFunc != nil {
		//return config.RoleExtractorFunc(ctx);
	}
	// logic to extract in default case
	return roles, err
}
