#!/usr/bin/env bash
# Author: Jayson Grace <jayson.e.grace@gmail.com>
# Provision logic for Odyssey creation.
set -ex

cleanup() {
    if [[ "${CLEANUP}" == "true" ]]; then
        # Remove build directory
        rm -rf "${PKR_BUILD_DIR}"

        # Remove unnecessary packages and files
        run_as_root apt-get autoremove -y
        run_as_root apt-get clean
        run_as_root rm -rf /var/lib/apt/lists/*
        run_as_root rm -rf /tmp/*
        run_as_root rm -rf /var/tmp/*

        # Remove pip cache
        python3 -m pip cache purge

        # Remove Python bytecode files
        run_as_root find / -type f -name "*.py[co]" -delete
        run_as_root find / -type d -name "__pycache__" -exec rm -rf {} +

        # Remove logs and other unnecessary files
        run_as_root find /var/log -type f -exec truncate -s 0 {} \;

        # Clean up cloud-init logs if running on EC2
        if [[ -n "${AWS_DEFAULT_REGION}" ]]; then
            run_as_root rm -rf /var/lib/cloud/instances/*
            run_as_root rm -f /var/log/cloud-init.log
            run_as_root rm -f /var/log/cloud-init-output.log
        fi

        # Remove bash history
        unset HISTFILE
        rm -f ~/.bash_history
        history -c
    fi
}

touch 'test.txt'
rm 'test.txt'
echo "YAY"

cleanup

exit 0
