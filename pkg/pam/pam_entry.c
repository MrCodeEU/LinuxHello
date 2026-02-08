// pam_entry.c - PAM module entry points
// This file provides the PAM module entry points that call into Go code

#include <security/pam_appl.h>
#include <security/pam_modules.h>
#include <stdlib.h>

// Import Go function - this is exported from the Go side
extern int goAuthenticate(pam_handle_t *pamh, int flags, int argc, char **argv);

// pam_send_message sends a text message to the user via the PAM conversation function
int pam_send_message(pam_handle_t *pamh, const char *message, int msg_style) {
    const struct pam_conv *conv;
    struct pam_message msg;
    const struct pam_message *msgp;
    struct pam_response *resp = NULL;

    if (pam_get_item(pamh, PAM_CONV, (const void **)&conv) != PAM_SUCCESS || conv == NULL) {
        return PAM_CONV_ERR;
    }

    msg.msg_style = msg_style;
    msg.msg = message;
    msgp = &msg;

    int ret = conv->conv(1, &msgp, &resp, conv->appdata_ptr);
    if (resp != NULL) {
        if (resp->resp != NULL) {
            free(resp->resp);
        }
        free(resp);
    }
    return ret;
}

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
