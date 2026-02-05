// pam_entry.c - PAM module entry points
// This file provides the PAM module entry points that call into Go code

#include <security/pam_appl.h>
#include <security/pam_modules.h>

// Import Go function - this is exported from the Go side
extern int goAuthenticate(pam_handle_t *pamh, int flags, int argc, char **argv);

// PAM authentication entry point
PAM_EXTERN int pam_sm_authenticate(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    return goAuthenticate(pamh, flags, argc, (char**)argv);
}

// PAM credential setting entry point
PAM_EXTERN int pam_sm_setcred(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)pamh; (void)flags; (void)argc; (void)argv;
    return PAM_SUCCESS;
}

// PAM account management entry point
PAM_EXTERN int pam_sm_acct_mgmt(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)pamh; (void)flags; (void)argc; (void)argv;
    return PAM_SUCCESS;
}

// PAM session open entry point
PAM_EXTERN int pam_sm_open_session(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)pamh; (void)flags; (void)argc; (void)argv;
    return PAM_SUCCESS;
}

// PAM session close entry point
PAM_EXTERN int pam_sm_close_session(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)pamh; (void)flags; (void)argc; (void)argv;
    return PAM_SUCCESS;
}

// PAM password change entry point
PAM_EXTERN int pam_sm_chauthtok(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)pamh; (void)flags; (void)argc; (void)argv;
    return PAM_SERVICE_ERR;
}
