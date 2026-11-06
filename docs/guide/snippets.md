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

## Editing a Snippet

The editor page shows the content of the snippet on the left and its
details on the right; on a narrow screen the details follow the content. The tag next to the title tells
whether sites can include the snippet or only insert it. Expand a row of the
snippet page to read a snippet without opening it.

The file name can change while nothing includes the snippet. Once a
configuration includes it, the name is fixed, so that configuration keeps
working.

The name, the description and the labels of variables and options can be
translated. A field holds the text of the interface language, and the
translation button next to it adds the other languages. Users of a language
without a translation see the English text.

### Variables

A variable makes a value differ from site to site, such as a target address
or a status code. It has a key, a label, a type (text, switch or select) and
a default value, and the content refers to it as <code v-pre>{{ .key }}</code>.

The quickest way to add one is to write the configuration with a real value
first, select the value and click **Make Variable**, or press <kbd>⌘E</kbd>
(<kbd>Ctrl+E</kbd> on Windows and Linux). The key and the type are proposed
from the line, the selected value becomes the default, and the content then
refers to the variable. Selecting `on` or `off` makes a switch.

In the content, every variable has its own color, the same as in the list of
variables, and pointing at one shows its type and default value. Typing
<code v-pre>{{ .</code> lists the variables to complete, and **New Variable…** at the end of
the list declares one where the cursor is. **Insert Variable** adds one at
the cursor. A variable the content uses without declaring it is marked, with
a button to declare it.

A snippet with variables is filled in by the config template panel and can
only be inserted. Nginx cannot include it, because the <code v-pre>{{ }}</code> placeholders
are not Nginx configuration.

### Preview

Below the content, **Preview** shows the form the config template panel asks
the user to fill in, next to the configuration it produces. It follows every
edit: changing a value updates the result and marks the lines that change,
and the preview tells whether the result is valid Nginx configuration, before
the snippet is saved. For a snippet without variables, the same place shows
the directive that includes it.

A line that holds only actions such as <code v-pre>{{ if .keepPath }}</code>, <code v-pre>{{ else }}</code> or
<code v-pre>{{ end }}</code> leaves no blank line in the result, so blocks can be written on
lines of their own.

## Templates {#built-in-templates}

The **Templates** tab of the snippet page lists the config templates that
come with Nginx UI, followed by those of enabled plugins, which are tagged
with the plugin. They are read only: expand a row to glance at one, or open
it to read it with its variables highlighted and try them in **Preview**.
**Copy as Snippet** opens a new snippet filled in from the template, to
change and save as your own.

A copy of a plugin template is a snippet like any other: it no longer
follows the plugin, and sites that include it get whatever it holds. Review
its directives before saving it.

## The File Header

The details are stored in a header at the top of the file. Its lines are
comments, so Nginx can always include the file:

```nginx [snippets/redirect.conf]
# Nginx UI Template Start
# name = "Redirect"
#
# [description]
# en = "Send every request to another address"
# zh_CN = "将所有请求转发到另一个地址"
#
# [variables.target]
# type = "string"
# name = { en = "Target address" }
# value = "https://example.com"
# Nginx UI Template End

return 301 {{ .target }}$request_uri;
```

The header uses the format of [Config Template](./nginx-ui-template.md),
written as comments, so it can also be edited by hand. A name in English
only is a plain string; a name with translations is a table of languages,
like the description.

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
