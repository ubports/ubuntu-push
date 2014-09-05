package accounts

/*
#include <libaccounts-glib/accounts-glib.h>

static void cb(AgManager *manager, AgAccountId account_id, gpointer p) {
    AgAccount *account = ag_manager_get_account(manager, account_id);
    if (!account) {
        return;
    }
    GList *services = ag_account_list_services(account);
    if (!services || !services->data) {
        return;
    }

    gocb();
}

void start() {
    AgManager *manager = ag_manager_new_for_service_type("ubuntuone");
    g_signal_connect(manager, "account-created", G_CALLBACK(cb), NULL);
    g_signal_connect(manager, "account-deleted", G_CALLBACK(cb), NULL);
    g_signal_connect(manager, "account-updated", G_CALLBACK(cb), NULL);

}

*/
import "C"
