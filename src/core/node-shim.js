if (globalThis.CustomEvent == null) {
  globalThis.CustomEvent = class CustomEvent extends Event {
    constructor (type, options = {}) {
      super(type, options)
      this.detail = options.detail
    }
  }
}

if (Promise.withResolvers == null) {
  Promise.withResolvers = () => {
    let resolve
    let reject
    const promise = new Promise((res, rej) => {
      resolve = res
      reject = rej
    })

    return { promise, resolve, reject }
  }
}
