export interface Container {
  message: string
  // eslint-disable-next-line ts/no-explicit-any
  args: Record<string, any>
}

function T(container: Container): string {
  return $gettext(container.message, container.args)
}

export default T
