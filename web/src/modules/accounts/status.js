export const IdentificationStatus = Object.freeze({
  Pending: "pending_identification",
  Identified: "identified",
});

export const PlatformLoginStatus = Object.freeze({
  Unknown: "unknown",
  Normal: "normal",
  NotLoggedIn: "not_logged_in",
  VerificationNeeded: "verification_needed",
  Expired: "expired",
  Restricted: "restricted",
  AccountMismatch: "account_mismatch",
  EnvironmentError: "environment_error",
});
