#!/usr/bin/env python3
"""
Brother Printer Health Monitor
Monitors printer responsiveness and attempts to wake it if unresponsive
"""

import os
import sys
import time
import requests
from datetime import datetime
from urllib3.exceptions import InsecureRequestWarning

# Suppress SSL warnings for self-signed certificates
requests.packages.urllib3.disable_warnings(category=InsecureRequestWarning)

# Configuration from environment variables
PRINTER_URL = os.getenv("PRINTER_URL")
PRINTER_PASSWORD = os.getenv("PRINTER_PASSWORD")
CHECK_INTERVAL = int(os.getenv("CHECK_INTERVAL", "300"))  # 5 minutes default
TIMEOUT = int(os.getenv("TIMEOUT", "10"))  # 10 seconds default
MAX_RETRIES = int(os.getenv("MAX_RETRIES", "3"))


class PrinterMonitor:
    def __init__(self, base_url, password, timeout=10):
        self.base_url = base_url.rstrip('/')  # Remove trailing slash
        self.password = password
        self.timeout = timeout
        self.status_url = f"{self.base_url}/general/status.html"
        self.info_url = f"{self.base_url}/general/information.html"

    def log(self, message, level="INFO"):
        timestamp = datetime.now().isoformat()
        print(f"[{timestamp}] [{level}] {message}", flush=True)

    def check_basic_connectivity(self):
        """Check if printer responds to basic HTTP request"""
        try:
            response = requests.get(
                self.base_url,
                timeout=self.timeout,
                verify=False,
                allow_redirects=True
            )
            return response.status_code in [200, 301, 302]
        except requests.exceptions.RequestException as e:
            self.log(f"Basic connectivity check failed: {e}", "ERROR")
            return False

    def check_status_page(self):
        """Check if printer status page is accessible"""
        try:
            response = requests.get(
                self.status_url,
                timeout=self.timeout,
                verify=False
            )
            if response.status_code == 200:
                content = response.text
                if "Sleep" in content:
                    self.log("Printer is in Sleep mode")
                    return "sleep"
                elif "Ready" in content:
                    self.log("Printer is Ready")
                    return "ready"
                else:
                    self.log("Printer status unknown")
                    return "unknown"
            return False
        except requests.exceptions.RequestException as e:
            self.log(f"Status page check failed: {e}", "ERROR")
            return False

    def get_printer_info(self):
        """Fetch detailed printer information"""
        try:
            response = requests.get(
                self.info_url,
                timeout=self.timeout,
                verify=False
            )
            if response.status_code == 200:
                content = response.text

                # Extract key information
                info = {}
                if "No Belt Unit" in content:
                    info["belt_error"] = True
                    self.log("⚠️  Belt Unit Error detected!", "WARNING")

                # Extract page counter if available
                if "Page&#32;Counter" in content:
                    # Simple extraction - could be enhanced
                    self.log("Printer information retrieved successfully")

                return info
        except requests.exceptions.RequestException as e:
            self.log(f"Failed to get printer info: {e}", "ERROR")
        return {}

    def wake_printer(self):
        """Attempt to wake printer by accessing status page"""
        self.log("Attempting to wake printer...")
        try:
            # Multiple quick requests can help wake the printer
            for i in range(3):
                requests.get(
                    self.status_url,
                    timeout=self.timeout,
                    verify=False
                )
                time.sleep(1)
            self.log("Wake signal sent")
            return True
        except requests.exceptions.RequestException as e:
            self.log(f"Failed to wake printer: {e}", "ERROR")
            return False

    def perform_health_check(self):
        """Perform complete health check"""
        self.log("=" * 60)
        self.log("Starting printer health check")
        self.log(f"Printer URL: {self.base_url}")

        # Check basic connectivity
        if not self.check_basic_connectivity():
            self.log("❌ Printer is not responding to network requests", "ERROR")
            self.log("This may indicate:")
            self.log("  - Printer is powered off")
            self.log("  - Network connectivity issues")
            self.log("  - Printer requires physical power cycle")
            return False

        self.log("✓ Basic network connectivity OK")

        # Check status
        status = self.check_status_page()
        if status in ["sleep", "ready"]:
            self.log(f"✓ Printer is responsive (Status: {status})")

            # Get detailed info
            info = self.get_printer_info()

            if info.get("belt_error"):
                self.log("⚠️  ACTION REQUIRED: Belt Unit error detected", "WARNING")
                self.log("Please reseat the belt unit:")
                self.log("  1. Open top cover")
                self.log("  2. Remove all drums/toners")
                self.log("  3. Remove and reseat belt unit")
                self.log("  4. Reinstall drums/toners")

            return True
        else:
            self.log("⚠️  Printer status unclear, attempting wake", "WARNING")
            self.wake_printer()

            # Wait and recheck
            time.sleep(5)
            status = self.check_status_page()
            if status:
                self.log("✓ Printer woke up successfully")
                return True
            else:
                self.log("❌ Printer did not respond after wake attempt", "ERROR")
                return False

    def run_continuous_monitoring(self):
        """Run continuous monitoring loop"""
        self.log("Starting continuous printer monitoring")
        self.log(f"Check interval: {CHECK_INTERVAL} seconds")

        while True:
            try:
                self.perform_health_check()
                self.log(f"Next check in {CHECK_INTERVAL} seconds")
                self.log("=" * 60)
                time.sleep(CHECK_INTERVAL)
            except KeyboardInterrupt:
                self.log("Monitoring stopped by user")
                break
            except Exception as e:
                self.log(f"Unexpected error: {e}", "ERROR")
                time.sleep(CHECK_INTERVAL)

    def run_single_check(self):
        """Run a single health check (for CronJob)"""
        success = self.perform_health_check()
        self.log("=" * 60)
        return 0 if success else 1


def main():
    # Validate required environment variables
    if not PRINTER_URL:
        print("ERROR: PRINTER_URL environment variable is required", file=sys.stderr)
        sys.exit(1)

    if not PRINTER_PASSWORD:
        print("ERROR: PRINTER_PASSWORD environment variable is required", file=sys.stderr)
        sys.exit(1)

    monitor = PrinterMonitor(PRINTER_URL, PRINTER_PASSWORD, TIMEOUT)

    # Check if running as CronJob (single check) or continuous
    run_mode = os.getenv("RUN_MODE", "cronjob")

    if run_mode == "continuous":
        monitor.run_continuous_monitoring()
    else:
        # CronJob mode - single check
        exit_code = monitor.run_single_check()
        sys.exit(exit_code)


if __name__ == "__main__":
    main()
