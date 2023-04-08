#!/bin/bash
set -ex

pkgs=$(go list ./...)

for pkg in $pkgs; do
    dir="$(basename "$pkg")/"
    if [[ "${dir}" != ".hooks/" ]] \
                              && [[ "${dir}" != "bin/" ]] \
                              && [[ "${dir}" != "cmd/" ]] \
                              && [[ "${dir}" != "config/" ]] \
                              && [[ "${dir}" != "deployments/" ]] \
                              && [[ "${dir}" != "files/" ]] \
                              && [[ "${dir}" != "images/" ]] \
                              && [[ "${dir}" != "logs/" ]] \
                              && [[ "${dir}" != "magefiles/" ]] \
                              && [[ "${dir}" != "modules/" ]] \
                              && [[ "${dir}" != "resources/" ]] \
                              && [[ "${dir}" != "templates/" ]] \
                              && [[ "${dir}" != "warpgate/" ]]; then
        go vet "${pkg}/${dir}"
    fi
done
