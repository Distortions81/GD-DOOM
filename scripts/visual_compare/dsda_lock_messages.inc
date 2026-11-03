/* Read-only message access for the private lock probe. Included after messenger.c. */
const char *GD_LockMessage(void) { return current_message ? current_message->str : ""; }
void GD_ClearLockMessage(void) { dsda_ClearMessages(); }
