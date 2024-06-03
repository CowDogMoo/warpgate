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
        run_as_root apt-get purge -y \
            git \
            gpg-agent \
            python3-pip \
            python3-setuptools \
            build-essential \
            libgmp-dev \
            manpages \
            man-db \
            bsdmainutils

        # Clean up cloud-init logs if running on EC2
        if [[ -n "${AWS_DEFAULT_REGION}" ]]; then
            run_as_root rm -rf /var/lib/cloud/instances/*
            run_as_root rm -f /var/log/cloud-init.log
            run_as_root rm -f /var/log/cloud-init-output.log
        fi

        # Remove temporary files
        run_as_root rm -rf /tmp/* /var/tmp/*

        # Remove build directory
        rm -rf "${PKR_BUILD_DIR}"

        # Remove any leftover logs
        run_as_root rm -rf /var/log/*
    fi
}

touch 'test.txt'
rm 'test.txt'
echo "YAY"

cleanup

exit 0
