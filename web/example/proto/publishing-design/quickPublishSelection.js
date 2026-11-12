import { accountAvailability } from "./accountAvailability.js";
import { resourceStatus } from "./model.js";

export function defaultPublishingResources(db, platform, category, accountId) {
  if (!platform || !category) return { account: null, asset: null };
  const eligible = db.accounts.filter(
    (account) => !accountAvailability(db, account, platform, category).reason,
  );
  const account =
    eligible.find((a) => a.id === accountId) || eligible[0] || null;
  const asset = account
    ? db.assets.find(
        (asset) =>
          asset.category === category &&
          !resourceStatus(db, asset, platform, [account.id]),
      ) || null
    : null;
  return { account, asset };
}
