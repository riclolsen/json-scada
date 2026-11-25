# install-lib.sh - shared helpers of the Linux installers (platform-ubuntu/platform-rhel
# json-scada-install*.sh). They make re-running an installer over an existing installation a clean
# upgrade: databases and local configuration are kept, the processes that were running are restarted.
#
# POSIX sh (the installers run under dash on Ubuntu). Source it from the platform directory after
# JS_USERNAME is set:
#   . ../platform-linux/install-lib.sh
#   js_begin
#
# Upgrade flow: git pull --recurse-submodules in the checkout, then run the same installer again.
# Set JS_ASSUME_YES=1 to answer the confirmations automatically.

JS_ROOT=$(cd .. && pwd)
JS_HOME=$(getent passwd "$JS_USERNAME" | cut -d: -f6)
JS_HOME=${JS_HOME:-/home/$JS_USERNAME}
JS_STATE_DIR=/var/lib/json-scada
JS_MANIFEST=$JS_STATE_DIR/config-manifest      # "<sha256> <path>" of each config file as installed
JS_RUNNING_FILE=$JS_STATE_DIR/running-programs # supervisor programs running before the upgrade
JS_COMMIT_FILE=$JS_STATE_DIR/installed-commit  # git commit of the last completed installation
JS_BACKUP_DIR=$JS_ROOT/backup/upgrade_$(date +%Y%m%d_%H%M%S)
JS_LOG=$JS_ROOT/log/upgrade.log
JS_IS_UPGRADE=0
JS_MONGO_HAD_DATA=0
JS_PG_HAS_DATA=0
JS_PG_KEEP_MAJOR=
JS_WARN_FILE=$(mktemp)

if command -v apt-get >/dev/null 2>&1; then
    JS_DISTRO=debian
    JS_SUP_DIR=/etc/supervisor/conf.d
    JS_PG_BASE=/var/lib/postgresql
else
    JS_DISTRO=rhel
    JS_SUP_DIR=/etc/supervisord.d
    JS_PG_BASE=/var/lib/pgsql
fi

js_log() {
    echo "[json-scada] $*"
    printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" | sudo tee -a "$JS_LOG" >/dev/null 2>&1
}

js_warn() {
    js_log "WARNING: $*"
    echo "$*" >> "$JS_WARN_FILE"
}

# js_confirm "question" Y|N : default answer when JS_ASSUME_YES is unset and there is no terminal
js_confirm() {
    [ "${JS_ASSUME_YES:-}" = 1 ] && return 0
    if ! (: < /dev/tty) 2>/dev/null; then
        [ "$2" = Y ]
        return
    fi
    if [ "$2" = Y ]; then printf '%s [Y/n] ' "$1"; else printf '%s [y/N] ' "$1"; fi
    read JS_ANSWER < /dev/tty
    case "$JS_ANSWER" in
        y|Y|yes|YES) return 0 ;;
        "") [ "$2" = Y ] ;;
        *) return 1 ;;
    esac
}

js_git() {
    sudo -u "$JS_USERNAME" git -C "$JS_ROOT" "$@"
}

# js_append_once FILE LINE [USER] : append LINE to FILE unless it is already there
js_append_once() {
    if [ -n "${3:-}" ]; then
        sudo -u "$3" sh -c 'grep -qxF "$2" "$1" 2>/dev/null || echo "$2" >> "$1"' sh "$1" "$2"
    else
        sudo sh -c 'grep -qxF "$2" "$1" 2>/dev/null || echo "$2" >> "$1"' sh "$1" "$2"
    fi
}

# ---------------------------------------------------------------------------------------------------
# Configuration files

js_sha() {
    sudo sha256sum "$1" 2>/dev/null | cut -d' ' -f1
}

# checksum of a supervisor program file without its autostart lines (stdin)
js_sha_na() {
    grep -v '^[[:space:]]*autostart[[:space:]]*=' | sha256sum | cut -d' ' -f1
}

js_manifest_get() {
    [ -f "$JS_MANIFEST" ] && awk -v p="$1" '$2 == p { h = $1 } END { print h }' "$JS_MANIFEST"
}

# js_record PATH : remember the checksum of PATH as installed by JSON-SCADA (for .ini files also the
# checksum without autostart lines, as PATH#na)
js_record() {
    JS_H=$(js_sha "$1")
    [ -n "$JS_H" ] || return 0
    JS_T=$(mktemp)
    if [ -f "$JS_MANIFEST" ]; then awk -v p="$1" '$2 != p && $2 != p "#na"' "$JS_MANIFEST" > "$JS_T"; fi
    echo "$JS_H $1" >> "$JS_T"
    case "$1" in *.ini) echo "$(sudo cat "$1" | js_sha_na) $1#na" >> "$JS_T" ;; esac
    sudo cp "$JS_T" "$JS_MANIFEST"
    rm -f "$JS_T"
}

js_backup_file() {
    [ "$JS_IS_UPGRADE" = 1 ] && [ -e "$1" ] || return 0
    sudo mkdir -p "$JS_BACKUP_DIR/files$(dirname "$1")"
    sudo cp -a "$1" "$JS_BACKUP_DIR/files$1"
}

# js_unmodified SRC DEST [na] : true when DEST was not changed locally, i.e. it matches the checksum
# recorded at the last installation or any version of SRC in the git history (installations made before
# the manifest existed). With "na", autostart lines are ignored.
js_unmodified() {
    if [ "${3:-}" = na ]; then
        JS_CUR=$(sudo cat "$2" 2>/dev/null | js_sha_na)
        JS_KEY="$2#na"
    else
        JS_CUR=$(js_sha "$2")
        JS_KEY=$2
    fi
    [ -n "$JS_CUR" ] || return 1
    [ "$JS_CUR" = "$(js_manifest_get "$JS_KEY")" ] && return 0
    case "$1" in /*) JS_ABS=$1 ;; *) JS_ABS=$(pwd)/$1 ;; esac
    JS_ABS=$(echo "$JS_ABS" | sed 's#/\./#/#g')
    JS_REL=${JS_ABS#"$JS_ROOT"/}
    # earlier locations of the source file: platform-rhel/ and platform-ubuntu/ merge the former
    # per-version platform-rhel9/, platform-rhel10/, platform-ubuntu-2404/ and platform-ubuntu-2604/
    case "$JS_REL" in
        platform-rhel/*) JS_RELS="$JS_REL platform-rhel9/${JS_REL#platform-rhel/} platform-rhel10/${JS_REL#platform-rhel/}" ;;
        platform-ubuntu/*) JS_RELS="$JS_REL platform-ubuntu-2404/${JS_REL#platform-ubuntu/} platform-ubuntu-2604/${JS_REL#platform-ubuntu/}" ;;
        *) JS_RELS=$JS_REL ;;
    esac
    for JS_R in $JS_RELS; do
        for JS_C in $(js_git log --format=%H -n 300 -- "$JS_R" 2>/dev/null); do
            if [ "${3:-}" = na ]; then
                JS_V=$(js_git show "$JS_C:$JS_R" 2>/dev/null | js_sha_na)
            else
                JS_V=$(js_git show "$JS_C:$JS_R" 2>/dev/null | sha256sum | cut -d' ' -f1)
            fi
            [ "$JS_V" = "$JS_CUR" ] && return 0
        done
    done
    return 1
}

# js_install_config SRC DEST : install a configuration file. On upgrades a locally changed DEST is kept
# and the new version is written next to it as DEST.jsnew (reported at the end); returns 2 then.
js_install_config() {
    if [ "$JS_IS_UPGRADE" = 1 ] && [ -f "$2" ] && ! cmp -s "$1" "$2"; then
        js_backup_file "$2"
        if js_unmodified "$1" "$2"; then
            sudo cp "$1" "$2"
            js_log "Updated $2"
        else
            sudo cp "$1" "$2.jsnew"
            js_warn "$2 has local changes and was kept; merge the new version from $2.jsnew"
            return 2
        fi
    else
        sudo cp "$1" "$2"
    fi
    sudo rm -f "$2.jsnew"
    js_record "$2"
}

# js_install_inis SRCDIR DESTDIR : install the supervisor program files. On upgrades a locally changed
# program file is replaced by the new version keeping its autostart setting (per [section]); program
# files of earlier versions that this version no longer ships are removed when unchanged.
js_install_inis() {
    for JS_SRC in "$1"/*.ini; do
        JS_DEST=$2/$(basename "$JS_SRC")
        if [ "$JS_IS_UPGRADE" = 1 ] && [ -f "$JS_DEST" ] && ! cmp -s "$JS_SRC" "$JS_DEST"; then
            js_backup_file "$JS_DEST"
            if js_unmodified "$JS_SRC" "$JS_DEST"; then
                sudo cp "$JS_SRC" "$JS_DEST"
            else
                JS_OLD=$(mktemp)
                JS_NEW=$(mktemp)
                sudo cat "$JS_DEST" > "$JS_OLD"
                awk -v old="$JS_OLD" '
                    function key(s) { gsub(/[ \t]/, "", s); return s }
                    BEGIN {
                        sec = ""
                        while ((getline l < old) > 0) {
                            if (l ~ /^[ \t]*\[/) sec = key(l)
                            else if (l ~ /^[ \t]*autostart[ \t]*=/) {
                                v = l; sub(/^[ \t]*autostart[ \t]*=[ \t]*/, "", v); sub(/[ \t;].*$/, "", v); a[sec] = v
                            }
                        }
                        sec = ""
                    }
                    /^[ \t]*\[/ { sec = key($0) }
                    /^[ \t]*autostart[ \t]*=/ && (sec in a) { print "autostart=" a[sec]; next }
                    { print }' "$JS_SRC" > "$JS_NEW"
                if ! js_unmodified "$JS_SRC" "$JS_DEST" na; then
                    js_warn "$JS_DEST had local changes besides autostart; replaced by the new version (autostart kept), previous file in $JS_BACKUP_DIR"
                fi
                sudo cp "$JS_NEW" "$JS_DEST"
                rm -f "$JS_OLD" "$JS_NEW"
            fi
            js_log "Updated $JS_DEST"
        else
            sudo cp "$JS_SRC" "$JS_DEST"
        fi
        js_record "$JS_DEST"
    done
    [ -f "$JS_MANIFEST" ] || return 0
    for JS_DEST in $(awk -v d="$2/" 'index($2, d) == 1 && $2 ~ /\.ini$/ { print $2 }' "$JS_MANIFEST"); do
        [ -f "$1/$(basename "$JS_DEST")" ] && continue
        if [ -f "$JS_DEST" ]; then
            js_backup_file "$JS_DEST"
            if [ "$(js_sha "$JS_DEST")" = "$(js_manifest_get "$JS_DEST")" ]; then
                sudo rm -f "$JS_DEST"
                js_log "Removed $JS_DEST (no longer shipped)"
            else
                js_warn "$JS_DEST is no longer shipped but has local changes; kept it"
                continue
            fi
        fi
        JS_T=$(mktemp)
        awk -v p="$JS_DEST" '$2 != p && $2 != p "#na"' "$JS_MANIFEST" > "$JS_T"
        sudo cp "$JS_T" "$JS_MANIFEST"
        rm -f "$JS_T"
    done
}

# ---------------------------------------------------------------------------------------------------
# Processes

js_stop_programs() {
    command -v supervisorctl >/dev/null 2>&1 || return 0
    JS_T=$(mktemp)
    {
        sudo cat "$JS_RUNNING_FILE" 2>/dev/null # left by an interrupted upgrade
        sudo supervisorctl status 2>/dev/null | awk '$2 == "RUNNING" { print $1 }'
    } | grep -v '^$' | sort -u > "$JS_T"
    sudo cp "$JS_T" "$JS_RUNNING_FILE"
    js_log "Running programs: $(tr '\n' ' ' < "$JS_T")"
    rm -f "$JS_T"
    sudo supervisorctl stop all >/dev/null 2>&1
    js_log 'Stopped all supervisor programs'
}

# reload supervisord (new program files, autostart programs) and start the programs running before
js_restore_programs() {
    sudo supervisorctl reload
    sleep 10
    for JS_P in $(sudo cat "$JS_RUNNING_FILE" 2>/dev/null); do
        JS_OUT=$(sudo supervisorctl start "$JS_P" 2>&1)
        case "$JS_OUT" in
            *started*|*RUNNING*) ;;
            *) js_warn "Program $JS_P was running before the upgrade and did not start: $JS_OUT" ;;
        esac
    done
    sudo rm -f "$JS_RUNNING_FILE"
    sudo supervisorctl status
}

# ---------------------------------------------------------------------------------------------------
# MongoDB

js_mongo_ready() {
    command -v mongosh >/dev/null 2>&1 || return 1
    JS_I=0
    while [ "$JS_I" -lt "${1:-30}" ]; do
        [ "$(mongosh --quiet --eval 'db.adminCommand({ping:1}).ok' 2>/dev/null)" = 1 ] && return 0
        JS_I=$((JS_I + 1))
        sleep 2
    done
    return 1
}

js_mongo_dbpath() {
    JS_P=$(awk '/^[ \t]*dbPath:/ { print $2 }' /etc/mongod.conf 2>/dev/null | head -n 1)
    echo "${JS_P:-/var/lib/mongodb}"
}

# true when the json_scada database exists (or mongod is down but had data before the installer ran)
js_mongo_has_data() {
    if js_mongo_ready 30; then
        [ "$(mongosh --quiet --eval 'db.getMongo().getDBNames().indexOf("json_scada") >= 0' 2>/dev/null)" = true ]
        return
    fi
    if [ "$JS_MONGO_HAD_DATA" = 1 ]; then
        js_warn 'MongoDB is not answering but has data; the demo database is not imported'
        return 0
    fi
    return 1
}

js_mongo_fcv() {
    js_mongo_ready "${2:-3}" || { js_log "MongoDB not answering $1, feature compatibility version not checked"; return 0; }
    JS_OUT=$(mongosh --quiet --file "$JS_ROOT/platform-linux/mongo-fcv.js" 2>&1)
    case "$JS_OUT" in
        *FCV_CURRENT*|*FCV_SET*) js_log "MongoDB $1: $JS_OUT" ;;
        *) js_warn "MongoDB $1: $JS_OUT" ;;
    esac
}

# existing database: feature compatibility version, new collections/indexes/validators
js_mongo_upgrade() {
    if ! js_mongo_ready 60; then
        js_warn "MongoDB does not answer after the upgrade (see: journalctl -u mongod). If its log reports an incompatible feature compatibility version, reinstall the previous MongoDB, set the FCV to that version and upgrade one major version at a time"
        return 0
    fi
    js_mongo_fcv 'after the upgrade' 30
    JS_OUT=$(mongosh --quiet json_scada < "$JS_ROOT/mongo_seed/b_create-db.js" 2>&1)
    JS_ERR=$(echo "$JS_OUT" | grep 'Error' | grep -v 'already exists\|NamespaceExists')
    if [ -n "$JS_ERR" ]; then js_warn "MongoDB schema update reported: $JS_ERR"; else js_log 'MongoDB schema updated (collections, indexes, validators)'; fi
}

# ---------------------------------------------------------------------------------------------------
# PostgreSQL

js_psql() {
    (cd / && sudo -u postgres psql -X -w -q "$@")
}

js_pg_datadir() {
    if [ "$JS_DISTRO" = debian ]; then echo "$JS_PG_BASE/$1/main"; else echo "$JS_PG_BASE/$1/data"; fi
}

# a new cluster has 3 databases (template0, template1, postgres), JSON-SCADA adds json_scada, grafanaappdb
# and metabaseappdb (database dirs in base/ are OIDs; pgsql_tmp does not count)
js_pg_has_userdb() {
    [ "$(sudo ls "$1/base" 2>/dev/null | grep -c '^[0-9][0-9]*$')" -gt 3 ]
}

# js_pg_prepare MAJOR : run before configuring PostgreSQL MAJOR. Sets JS_PG_HAS_DATA, and
# JS_PG_KEEP_MAJOR when the data is in a cluster of another major version: that cluster keeps serving
# JSON-SCADA, the installer must not initialize, configure or start the MAJOR cluster.
js_pg_prepare() {
    JS_PG_KEEP_MAJOR=
    JS_PG_HAS_DATA=0
    if js_pg_has_userdb "$(js_pg_datadir "$1")"; then
        JS_PG_HAS_DATA=1
        return 0
    fi
    for JS_D in "$JS_PG_BASE"/*; do
        JS_M=$(basename "$JS_D")
        case "$JS_M" in ''|*[!0-9]*) continue ;; esac
        [ "$JS_M" = "$1" ] && continue
        if js_pg_has_userdb "$(js_pg_datadir "$JS_M")"; then
            if [ -z "$JS_PG_KEEP_MAJOR" ] || [ "$JS_M" -gt "$JS_PG_KEEP_MAJOR" ]; then JS_PG_KEEP_MAJOR=$JS_M; fi
        fi
    done
    [ -n "$JS_PG_KEEP_MAJOR" ] || return 0
    JS_PG_HAS_DATA=1
    if [ "$JS_DISTRO" = debian ]; then
        js_warn "The PostgreSQL data is in the $JS_PG_KEEP_MAJOR/main cluster, this version installs PostgreSQL $1. The $JS_PG_KEEP_MAJOR cluster keeps serving JSON-SCADA unchanged. To migrate (with TimescaleDB installed for both versions): sudo pg_dropcluster $1 main --stop; sudo pg_upgradecluster $JS_PG_KEEP_MAJOR main; then run this installer again"
    else
        js_warn "The PostgreSQL data is in /var/lib/pgsql/$JS_PG_KEEP_MAJOR/data, this version installs PostgreSQL $1. The postgresql-$JS_PG_KEEP_MAJOR service keeps serving JSON-SCADA unchanged. Migrate with /usr/pgsql-$1/bin/pg_upgrade (TimescaleDB installed for both versions), then run this installer again"
    fi
}

# existing database: TimescaleDB extensions to the installed version
js_pg_upgrade() {
    JS_I=0
    while [ "$JS_I" -lt 30 ] && ! js_psql -tAc 'SELECT 1' >/dev/null 2>&1; do JS_I=$((JS_I + 1)); sleep 2; done
    if [ "$(js_psql -tAc "SELECT 1 FROM pg_database WHERE datname = 'json_scada'" 2>/dev/null)" != 1 ]; then
        js_warn 'PostgreSQL does not answer or has no json_scada database; extensions not updated'
        return 0
    fi
    for JS_E in timescaledb timescaledb_toolkit; do
        [ "$(js_psql -d json_scada -tAc "SELECT 1 FROM pg_extension WHERE extname = '$JS_E'" 2>/dev/null)" = 1 ] || continue
        # must be the first command of the session (-X: no psqlrc)
        if JS_OUT=$(js_psql -d json_scada -c "ALTER EXTENSION $JS_E UPDATE;" 2>&1); then
            js_log "PostgreSQL extension $JS_E updated"
        else
            js_warn "ALTER EXTENSION $JS_E UPDATE failed: $JS_OUT"
        fi
    done
}

# ---------------------------------------------------------------------------------------------------

js_detect() {
    if [ -f "$JS_COMMIT_FILE" ] || [ -f "$JS_SUP_DIR/server_realtime_auth.ini" ] || [ -x "$JS_ROOT/bin/calculations" ]; then
        JS_IS_UPGRADE=1
    fi
    if sudo test -f "$(js_mongo_dbpath)/WiredTiger"; then
        JS_IS_UPGRADE=1
        JS_MONGO_HAD_DATA=1
    fi
}

js_begin() {
    sudo mkdir -p "$JS_STATE_DIR"
    sudo -u "$JS_USERNAME" mkdir -p "$JS_ROOT/log"
    js_detect
    if [ "$JS_IS_UPGRADE" != 1 ]; then
        js_log "Fresh installation in $JS_ROOT"
        return 0
    fi

    JS_PREV=$(cat "$JS_COMMIT_FILE" 2>/dev/null)
    JS_HEAD=$(js_git rev-parse HEAD 2>/dev/null)
    echo ""
    echo "An existing JSON-SCADA installation was found: it will be upgraded."
    echo "Databases, conf/, displays and locally changed system configuration files are kept,"
    echo "supervisor programs are stopped and the running ones restarted at the end."
    echo "Backups go to $JS_BACKUP_DIR, the log to $JS_LOG."
    if [ -n "$JS_PREV" ] && [ -n "$JS_HEAD" ] && ! js_git merge-base --is-ancestor "$JS_PREV" "$JS_HEAD" 2>/dev/null; then
        echo "WARNING: this checkout ($JS_HEAD) does not contain the installed version ($JS_PREV):"
        echo "it is older or diverged. Downgrades are not supported."
        js_confirm 'Continue anyway?' N || exit 1
    else
        js_confirm 'Continue?' Y || exit 1
    fi
    js_log "Upgrading $JS_ROOT from ${JS_PREV:-an unrecorded version} to ${JS_HEAD:-unknown}"

    sudo mkdir -p "$JS_BACKUP_DIR"
    [ -d "$JS_ROOT/conf" ] && sudo cp -a "$JS_ROOT/conf" "$JS_BACKUP_DIR/conf"
    [ -d "$JS_SUP_DIR" ] && sudo cp -a "$JS_SUP_DIR" "$JS_BACKUP_DIR/supervisor-programs"
    js_log "conf/ and supervisor programs backed up to $JS_BACKUP_DIR"

    js_stop_programs
    # lets the MongoDB packages of the next major version open the data
    js_mongo_fcv 'before the upgrade'
}

js_finish() {
    [ "$JS_IS_UPGRADE" = 1 ] && js_restore_programs
    JS_HEAD=$(js_git rev-parse HEAD 2>/dev/null)
    [ -n "$JS_HEAD" ] && echo "$JS_HEAD" | sudo tee "$JS_COMMIT_FILE" >/dev/null
    sudo chown -R "$JS_USERNAME": "$JS_ROOT/backup" "$JS_LOG" 2>/dev/null
    echo ""
    if [ "$JS_IS_UPGRADE" = 1 ]; then
        js_log "Upgrade complete. Backups: $JS_BACKUP_DIR"
    else
        js_log 'Installation complete'
    fi
    if [ -s "$JS_WARN_FILE" ]; then
        echo "Review these warnings (also in $JS_LOG):"
        sed 's/^/  - /' "$JS_WARN_FILE"
    fi
    rm -f "$JS_WARN_FILE"
}
