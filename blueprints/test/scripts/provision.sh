#!/usr/bin/env bash
# Author: Jayson Grace <jayson.e.grace@gmail.com>
# Provision logic for Odyssey creation.
set -ex

cleanup() {
    if [[ "${CLEANUP}" == "true" ]]; then
        # Clean up apt cache
        run_as_root apt-get clean
        run_as_root rm -rf /var/lib/apt/lists/*

        # Clean up pip cache
        python3 -m pip cache purge

        # Remove unused packages and their dependencies
        run_as_root apt-get autoremove -y

        # Check if packages exist before purging them
        for package in git gpg-agent libgmp-dev manpages man-db bsdmainutils; do
            if dpkg -s $package &> /dev/null; then
                run_as_root apt-get purge -y $package
            else
                echo "Package $package is not installed, skipping."
            fi
        done

        # Clean up cloud-init logs if running on EC2
        if [[ -n "${AWS_DEFAULT_REGION}" ]]; then
            run_as_root rm -rf /var/lib/cloud/instances/*
            run_as_root rm -f /var/log/cloud-init.log
            run_as_root rm -f /var/log/cloud-init-output.log
        fi

        # Remove temporary files
        run_as_root rm -rf /tmp/* /var/tmp/*

        # Remove any leftover logs
        run_as_root rm -rf /var/log/*
    fi
}

touch 'test.txt'
rm 'test.txt'
echo "YAY"

cleanup

exit 0
