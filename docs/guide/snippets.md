---
outline: [2, 3]
---

# Snippets

A snippet keeps a piece of Nginx configuration in one place, such as cache
rules, security headers or the PHP handler of your sites. Sites include it,
so changing the snippet changes every site that uses it. Snippets are listed
under **Manage Sites > Snippets**.

## Where Snippets Live

Every snippet is a file in the `snippets` directory of the Nginx
configuration directory, for example `/etc/nginx/snippets/static-cache.conf`.
This is where many distributions already keep theirs, so a snippet that came
with Nginx, such as `snippets/fastcgi-php.conf` on Debian, is listed too.

A file is a snippet when its name ends in `.conf` and uses only letters,
digits, dots, dashes and underscores. The `snippets/plugins` directory is
reserved and left alone.

## Using a Snippet

There are two ways to use a snippet in a site:

- **Include it.** Add the directive the snippet page shows to a `server` or
  `location` block:

  ```nginx
  location /assets/ {
      include snippets/static-cache.conf;
  }
  ```

  The site follows every later change of the snippet.
- **Insert it.** In the site editor, open the config template panel, choose
  the snippet and click **Insert**. Its content is copied into the site, which
  does not follow later changes. **Include** in the same dialog adds the
  directive for you.

Saving a snippet tests the whole configuration with `nginx -t` and reloads
Nginx. When Nginx rejects the configuration, the previous snippet is kept and
the error is shown.

::: warning
A snippet that a site still includes cannot be deleted, since Nginx would
reject the configuration without it. The snippet page lists the files that
include it.
:::

## Name, Description and Variables

The name and description shown on the snippet page are stored in a header at
the top of the file. Its lines are comments, so Nginx can always include the
file:

```nginx [snippets/static-cache.conf]
# Nginx UI Template Start
# name = "Static file cache"
#
# [description]
# en = "Cache images, scripts and styles for a week"
# Nginx UI Template End

expires 7d;
add_header Cache-Control "public";
```

The header uses the format of [Config Template](./nginx-ui-template.md),
written as comments. A snippet can declare variables in it the same way:

```nginx [snippets/hsts.conf]
# Nginx UI Template Start
# name = "HSTS"
#
# [variables.maxAge]
# type = "string"
# name = { en = "Max Age" }
# value = "31536000"
# Nginx UI Template End

add_header Strict-Transport-Security "max-age={{ .maxAge }}" always;
```

A snippet with variables is filled in by the config template panel and can
only be inserted. Nginx cannot include it, because the `{{ }}` placeholders
are not Nginx configuration. Variables are added by editing the header of the
file, for example in **Manage Configs**; the snippet page keeps them.

## Synchronizing to Nodes

**Synchronize** on the snippet page copies every snippet to the nodes you
choose and keeps them up to date: a saved snippet is copied to them, and a
deleted one is removed from them. Choose whether snippets that already exist
on a node are replaced.

A site synchronized to a node brings the snippets it includes along. They
are created on the node only where they are missing, never replacing the
copy of the node, since a snippet that came with the distribution of a node
may need to differ. Synchronize the snippets themselves to replace them.

::: tip
Synchronizing snippets uses the directory deployment of
[Manage Multi-Host Nginx with Cluster](./manage-multi-host-nginx-with-cluster.md),
so the snippets directory shows the same targets in **Manage Configs**.
:::
