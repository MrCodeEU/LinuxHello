#!/bin/bash

# LinuxHello PAM Management Script

PAM_FILE="/etc/pam.d/sudo"
PAM_LINE="auth        sufficient    /usr/local/lib/security/pam_linuxhello.so"
BACKUP_FILE="/etc/pam.d/sudo.backup.linuxhello"

enable_pam() {
    # Create backup if it doesn't exist
    if [ ! -f "$BACKUP_FILE" ]; then
        cp "$PAM_FILE" "$BACKUP_FILE"
    fi
    
    # Check if PAM line is already enabled
    if grep -q "^auth.*pam_linuxhello.so" "$PAM_FILE"; then
        echo "PAM module is already enabled"
        return 0
    fi
    
    # Check if PAM line exists but is commented
    if grep -q "# auth.*pam_linuxhello.so" "$PAM_FILE"; then
        # Uncomment the line
        sed -i 's/# auth\(.*pam_linuxhello.so\)/auth\1/' "$PAM_FILE"
        echo "PAM module enabled (uncommented)"
    else
        # Add the PAM line at the beginning of auth section
        sed -i "/^auth/i\\$PAM_LINE" "$PAM_FILE"
        echo "PAM module enabled (added)"
    fi
    
    echo "LinuxHello PAM module is now active for sudo authentication"
}

disable_pam() {
    if grep -q "^auth.*pam_linuxhello.so" "$PAM_FILE"; then
        # Comment out the line instead of removing it
        sed -i 's/^auth\(.*pam_linuxhello.so\)/# auth\1/' "$PAM_FILE"
        echo "PAM module disabled (commented out)"
    else
        echo "PAM module is already disabled"
    fi
}

status_pam() {
    if grep -q "^auth.*pam_linuxhello.so" "$PAM_FILE"; then
        echo "enabled"
    else
        echo "disabled"
    fi
}

case "$1" in
    enable)
        enable_pam
        ;;
    disable)
        disable_pam
        ;;
    status)
        status_pam
        ;;
    *)
        echo "Usage: $0 {enable|disable|status}"
        exit 1
        ;;
esac