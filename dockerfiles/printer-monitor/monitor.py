#!/usr/bin/env python3
"""
Brother Printer Health Monitor
Monitors printer responsiveness and attempts to wake it if unresponsive
"""

import os
import re
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
        self.sleep_url = f"{self.base_url}/general/sleep.html"
        self.session = requests.Session()  # Use session to maintain cookies

    def log(self, message, level="INFO"):
        timestamp = datetime.now().isoformat()
        print(f"[{timestamp}] [{level}] {message}", flush=True)

    def login(self):
        """Login to printer admin interface"""
        try:
            login_data = {
                'B12a1': self.password,
                'loginurl': '/general/sleep.html'
            }
            response = self.session.post(
                self.status_url,
                data=login_data,
                timeout=self.timeout,
                verify=False,  # nosemgrep: python.requests.security.disabled-cert-validation.disabled-cert-validation
                allow_redirects=False  # Don't auto-redirect so we can check for passerror
            )

            # Check if login was successful (should redirect to our target page, not passerror)
            if response.status_code in [301, 302]:
                location = response.headers.get('Location', '')
                if 'passerror' in location:
                    self.log("Login failed: incorrect password", "ERROR")
                    return False
                elif '/general/sleep.html' in location:
                    self.log("✓ Logged in to printer admin interface")
                    # Follow the redirect manually
                    self.session.get(f"{self.base_url}{location}", verify=False, timeout=self.timeout)  # nosemgrep: python.requests.security.disabled-cert-validation.disabled-cert-validation
                    return True

            if response.status_code == 200:
                self.log("✓ Logged in to printer admin interface")
                return True
            else:
                self.log(f"Login failed with status {response.status_code}", "ERROR")
                return False
        except requests.exceptions.RequestException as e:
            self.log(f"Login failed: {e}", "ERROR")
            return False

    def configure_sleep_time(self, minutes):
        """Configure printer sleep time in minutes"""
        try:
            # First get the sleep page to extract CSRF token
            response = self.session.get(
                self.sleep_url,
                timeout=self.timeout,
                verify=False  # nosemgrep: python.requests.security.disabled-cert-validation.disabled-cert-validation
            )

            if response.status_code != 200:
                self.log("Failed to access sleep settings page", "ERROR")
                return False

            # Extract CSRF token from the page
            csrf_match = re.search(r'name="CSRFToken" value="([^"]+)"', response.text, re.DOTALL)
            if not csrf_match:
                self.log("Failed to extract CSRF token", "ERROR")
                return False

            # Get the token and preserve its exact format (including newlines)
            csrf_token = csrf_match.group(1)

            # Submit the new sleep time
            sleep_data = {
                'pageid': '6',
                'CSRFToken': csrf_token,
                'postif_registration_reject': '1',
                'B1d': str(minutes)
            }

            response = self.session.post(
                self.sleep_url,
                data=sleep_data,
                timeout=self.timeout,
                verify=False,  # nosemgrep: python.requests.security.disabled-cert-validation.disabled-cert-validation
                allow_redirects=False
            )

            # Follow any redirects
            if response.status_code in [301, 302]:
                location = response.headers.get('Location', '')
                if location:
                    response = self.session.get(f"{self.base_url}{location}", verify=False, timeout=self.timeout)  # nosemgrep: python.requests.security.disabled-cert-validation.disabled-cert-validation

            if response.status_code == 200:
                # Verify the change by reading back the sleep page
                verify_response = self.session.get(
                    self.sleep_url,
                    timeout=self.timeout,
                    verify=False  # nosemgrep: python.requests.security.disabled-cert-validation.disabled-cert-validation
                )
                value_match = re.search(r'name="B1d"[^>]*value="(\d+)"', verify_response.text)
                if value_match:
                    actual_value = value_match.group(1)
                    self.log(f"✓ Sleep time configured to {actual_value} minutes (requested: {minutes})")
                else:
                    self.log(f"✓ Sleep time configuration submitted (requested: {minutes} minutes)")
                return True
            else:
                self.log(f"Failed to configure sleep time: status {response.status_code}", "ERROR")
                return False

        except Exception as e:
            self.log(f"Failed to configure sleep time: {e}", "ERROR")
            return False

    def check_basic_connectivity(self):
        """Check if printer responds to basic HTTP request"""
        try:
            response = requests.get(
                self.base_url,
                timeout=self.timeout,
                verify=False,  # nosemgrep: python.requests.security.disabled-cert-validation.disabled-cert-validation
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
                verify=False  # nosemgrep: python.requests.security.disabled-cert-validation.disabled-cert-validation
            )
            if response.status_code == 200:
                content = response.text
                # Look for actual status in the moni_data div, not menu items
                status_match = re.search(r'<span class="moni[^"]*">([^<]+)</span>', content)
                if status_match:
                    status_text = status_match.group(1).strip()
                    if "Sleep" in status_text:
                        self.log("Printer is in Sleep mode")
                        return "sleep"
                    elif "Ready" in status_text:
                        self.log("Printer is Ready")
                        return "ready"
                    else:
                        self.log(f"Printer status: {status_text}")
                        return "ready"  # Treat other statuses as ready
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
                verify=False  # nosemgrep: python.requests.security.disabled-cert-validation.disabled-cert-validation
            )
            if response.status_code == 200:
                content = response.text

                # Extract key information
                info = {}

                # Exclude Error History section to avoid false positives from historical errors
                # Split content at Error History section if it exists
                if "Error&#32;History" in content or "Error History" in content:
                    # Find the Error History section and exclude it from our checks
                    error_history_start = content.find("Error&#32;History")
                    if error_history_start == -1:
                        error_history_start = content.find("Error History")

                    if error_history_start != -1:
                        # Only check content before Error History
                        current_status_content = content[:error_history_start]
                    else:
                        current_status_content = content
                else:
                    current_status_content = content

                # Check for current belt unit errors (not historical)
                if "No Belt Unit" in current_status_content:
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
        """Attempt to wake printer using multiple methods (no blank pages)"""
        self.log("Attempting to wake printer...")

        # Method 1: Connect to JetDirect port without sending data
        try:
            import socket
            from urllib.parse import urlparse

            parsed = urlparse(self.base_url)
            host = parsed.hostname
            port = 9100

            self.log("Method 1: Connecting to JetDirect port (no data)...")
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(5)
            sock.connect((host, port))
            sock.close()
            self.log("✓ JetDirect connection made")
        except Exception as e:
            self.log(f"JetDirect connection failed: {e}", "WARNING")

        # Method 2: HTTP requests to web interface
        try:
            self.log("Method 2: Sending HTTP wake signals...")
            pages = [self.status_url, self.info_url, self.base_url]
            for round in range(3):
                for page in pages:
                    try:
                        requests.get(
                            page,
                            timeout=self.timeout,
                            verify=False  # nosemgrep: python.requests.security.disabled-cert-validation.disabled-cert-validation
                        )
                    except:
                        pass
                time.sleep(0.5)

            self.log("✓ HTTP wake signals sent")
            return True

        except Exception as e:
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
        if status == "ready":
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
        elif status == "sleep":
            self.log("Printer is in sleep mode")

            # First, try to wake the printer
            self.log("Attempting to wake printer from sleep...")
            self.wake_printer()

            # Wait for printer to wake
            self.log("Waiting 15 seconds for printer to wake...")
            time.sleep(15)

            # Check if it woke up
            status = self.check_status_page()
            if status == "ready":
                self.log("✓ Printer successfully woke up!")
            else:
                self.log("⚠️  Printer did not fully wake, but continuing to configure sleep timer...")

            # Configure sleep timer to prevent going back to sleep
            self.log("Configuring sleep timer to prevent auto-sleep...")
            if self.login():
                if self.configure_sleep_time(30):
                    self.log("✓ Sleep timer updated - printer will stay awake for 30 minutes")

                    # Get detailed info
                    info = self.get_printer_info()
                    if info.get("belt_error"):
                        self.log("⚠️  ACTION REQUIRED: Belt Unit error detected", "WARNING")
                        self.log("Please reseat the belt unit:")
                        self.log("  1. Open top cover")
                        self.log("  2. Remove all drums/toners")
                        self.log("  3. Remove and reseat belt unit")
                        self.log("  4. Reinstall drums/toners")

                    # Final status check
                    final_status = self.check_status_page()
                    if final_status == "ready":
                        self.log("✓ Printer is now READY and configured")
                        return True
                    else:
                        self.log(f"⚠️  Printer status: {final_status}", "WARNING")
                        return True  # Still return True as config was successful
                else:
                    self.log("⚠️  Failed to configure sleep time", "WARNING")
                    return False
            else:
                self.log("⚠️  Failed to login to printer", "WARNING")
                return False
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
