export default {
  400: () => $gettext('Code completion is not enabled'),
  40101: () => $gettext('The provider rejected the API token'),
  40102: () => $gettext('The endpoint does not provide a model list'),
  40103: () => $gettext('Timed out connecting to the endpoint'),
  40104: () => $gettext('The endpoint returned an error: {0}'),
  40105: () => $gettext('Unable to reach the endpoint: {0}'),
  40106: () => $gettext('Enter the API token again to list models from a different endpoint'),
  40201: () => $gettext('Model {0} is not enabled for the assistant'),
  40202: () => $gettext('Thinking level {0} is not configured for model {1}'),
  40203: () => $gettext('Model {0} is listed more than once'),
  40204: () => $gettext('Unknown thinking preset: {0}'),
  40205: () => $gettext('Unknown thinking level: {0}'),
}
