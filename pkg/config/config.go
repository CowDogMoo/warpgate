package config

// Config is the struct that holds the configuration for the application.
//
// **Attributes:**
//
// Debug: A boolean flag to enable debug mode.
// Log: The configuration for the logger.
type Config struct {
	Debug bool      `mapstructure:"debug"`
	Log   LogConfig `mapstructure:"log"`
}

// LogConfig stores the configuration for the logger.
//
// **Attributes:**
//
// Format: The format for the log messages.
// Level: The logging level.
// LogPath: The path to the log file.
type LogConfig struct {
	Format  string `mapstructure:"format"`
	Level   string `mapstructure:"level"`
	LogPath string `mapstructure:"log_path"`
}
