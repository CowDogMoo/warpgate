/*
Copyright © 2024-present, Jayson Grace <jayson.e.grace@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
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
