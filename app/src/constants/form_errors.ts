export default {
  required: () => $gettext('This field should not be empty'),
  email: () => $gettext('This field should be a valid email address'),
  db_unique: () => $gettext('This value is already taken'),
  hostname: () => $gettext('This field should be a valid hostname'),
  safety_text: () => $gettext('This field should only contain letters, unicode characters, numbers, and -_.'),
}
