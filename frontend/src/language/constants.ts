import gettext from '@/gettext'

const {$gettext} = gettext

export const msg = [
    $gettext('The username or password is incorrect'),
    $gettext('Prohibit changing root password in demo'),
    $gettext('Prohibit deleting the default user'),
    $gettext('Failed to get certificate information'),

    $gettext('Generating private key for registering account'),
    $gettext('Preparing lego configurations'),
    $gettext('Creating client facilitates communication with the CA server'),
    $gettext('Using HTTP01 challenge provider'),
    $gettext('Using DNS01 challenge provider'),
    $gettext('Setting environment variables'),
    $gettext('Cleaning environment variables'),
    $gettext('Registering user'),
    $gettext('Obtaining certificate'),
    $gettext('Writing certificate to disk'),
    $gettext('Writing certificate private key to disk'),
    $gettext('Reloading nginx'),
    $gettext('Finished'),
    $gettext('Issued certificate successfully'),

    $gettext('Initialing core upgrader'),
    $gettext('Initial core upgrader error'),
    $gettext('Downloading latest release'),
    $gettext('Download latest release error'),
    $gettext('Performing core upgrade'),
    $gettext('Perform core upgrade error'),
    $gettext('Upgraded successfully'),

    $gettext('File exists'),
    $gettext('Requested with wrong parameters')
]
