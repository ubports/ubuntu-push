#!/bin/sh
set -eu

PROJECT=launchpad.net/ubuntu-push

mktpl () {
    for f in GoFiles CgoFiles; do
        echo -n '{{join .'$f' "\\n"}}'
    done
}

directs () {
    go list -f "$(mktpl)" $1 | sed -e "s|^|$1|"
}

indirects () {
    for i in $(go list -f '{{join .Deps "\n"}}' $1 | grep ^$PROJECT ); do
        directs $i/
    done
    wait
}

norm () {
    tr "\n" " " | sed -r -e "s|$PROJECT/?||g" -e 's/ *$//'
}

out=".${1%.go}.deps"
( echo -n "${1%.go} ${out}: "; indirects $(echo $1 | norm) | norm ) > "$out"

