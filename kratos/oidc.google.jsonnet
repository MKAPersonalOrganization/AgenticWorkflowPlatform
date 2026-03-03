// Google OIDC claims → Kratos identity traits mapper
// Docs: https://www.ory.sh/docs/kratos/social-sign-in/google
local claims = std.extVar('claims');

{
  identity: {
    traits: {
      email: claims.email,
      [if std.objectHas(claims, 'given_name') || std.objectHas(claims, 'family_name') then 'name']: {
        [if std.objectHas(claims, 'given_name') then 'first']: claims.given_name,
        [if std.objectHas(claims, 'family_name') then 'last']: claims.family_name,
      },
      [if std.objectHas(claims, 'picture') then 'picture']: claims.picture,
    },
  },
}
