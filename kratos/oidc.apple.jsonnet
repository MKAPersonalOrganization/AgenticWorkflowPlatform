// Apple OIDC claims → Kratos identity traits mapper
// Docs: https://www.ory.sh/docs/kratos/social-sign-in/apple
// Note: Apple only returns name on the FIRST authorization; subsequent logins
// will only have 'email' and 'sub'. The mapper must handle missing fields.
local claims = std.extVar('claims');

{
  identity: {
    traits: {
      email: claims.email,
      [if std.objectHas(claims, 'given_name') || std.objectHas(claims, 'family_name') then 'name']: {
        [if std.objectHas(claims, 'given_name') then 'first']: claims.given_name,
        [if std.objectHas(claims, 'family_name') then 'last']: claims.family_name,
      },
    },
  },
}
