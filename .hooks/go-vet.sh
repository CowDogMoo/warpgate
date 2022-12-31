#!/bin/bash
set -ex

pkg=$(go list ./...)
for dir in */; do
    if [[ "${dir}" != ".mage" ]] \
                              && [[ "${dir}" != ".hooks/" ]] \
                              && [[ "${dir}" != "config/" ]] \
                              && [[ "${dir}" != "magefiles/" ]] \
                              && [[ "${dir}" != "blueprints/" ]] \
                              && [[ "${dir}" != "cmd/" ]] \
                              && [[ "${dir}" != "templates/" ]] \
                              && [[ "${dir}" != "docs/" ]] \
                              && [[ "${dir}" != "dist/" ]] \
                              && [[ "${dir}" != "images/" ]] \
                              && [[ "${dir}" != "resources/" ]] \
                              && [[ "${dir}" != "files/" ]] \
                              && [[ "${dir}" != "logs/" ]]; then
        go vet "${pkg}/${dir}"
    fi
done
