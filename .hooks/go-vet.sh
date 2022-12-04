#!/bin/bash
set -ex

pkg=$(go list ./...)
for dir in */; do
    if [[ "${dir}" != ".mage" ]] \
                              && [[ "${dir}" != "config/" ]] \
                              && [[ "${dir}" != "bin/" ]] \
                              && [[ "${dir}" != "magefiles/" ]] \
                           && [[ "${dir}" != "resources/" ]] \
                                 && [[ "${dir}" != "docs/" ]] \
                            && [[ "${dir}" != "files/" ]] \
                             && [[ "${dir}" != "logs/" ]]; then
        go vet "${pkg}/${dir}"
    fi
done
