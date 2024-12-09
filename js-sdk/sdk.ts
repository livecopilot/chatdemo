import { ICloseEvent, IMessageEvent, w3cwebsocket } from 'websocket'

export const Ping = new Uint8Array([0, 100, 0, 0, 0, 0])
export const Pong = new Uint8Array([0, 101, 0, 0, 0, 0])

const heartbeatInterval = 10 // second

export let sleep = async (second: number) =>
  new Promise(resolve => setTimeout(resolve, second * 1000))

export enum State {
  INIT,
  CONNECTING,
  CONNECTED,
  RECONNECTING,
  CLOSING,
  CLOSED,
}

export enum Ack {
  Success = 'Success',
  Timeout = 'Timeout',
  LoginFailed = 'LoginFailed',
  LoginEd = 'LoginEd',
}

export let doLogin = async (
  url: string,
): Promise<{ status: string; conn: w3cwebsocket }> => {
  const LoginTimeout = 5 // second

  return new Promise((resolve, reject) => {
    let conn = new w3cwebsocket(url)
    conn.binaryType = 'arraybuffer'

    let tr = setTimeout(() => {
      resolve({ status: Ack.Timeout, conn: conn })
    }, LoginTimeout * 1000)

    conn.onopen = () => {
      console.info('websocket open -- readyState: ', conn.readyState)

      if (conn.readyState === w3cwebsocket.OPEN) {
        clearTimeout(tr)
        resolve({ status: Ack.Success, conn: conn })
      }
    }

    conn.onerror = err => {
      clearTimeout(tr)
      resolve({ status: Ack.LoginFailed, conn: conn })
    }
  })
}

export class IMClient {
  wsurl: string = ''
  state = State.INIT

  private conn: w3cwebsocket | null = null
  private lastRead: number = 0

  constructor(url: string, user: string) {
    this.wsurl = `${url}?user=${user}`
    this.conn = null
    this.lastRead = Date.now()
  }

  // login
  async login(): Promise<{ status: string }> {
    if (this.state == State.CONNECTED) {
      return { status: Ack.LoginEd }
    }
    this.state = State.CONNECTING

    let { status, conn } = await doLogin(this.wsurl)
    console.log('login -', status)

    if (status !== Ack.Success) {
      this.state = State.INIT

      return { status }
    }

    conn.onmessage = (evt: IMessageEvent) => {
      try {
        this.lastRead = Date.now()

        let buf = Buffer.from(<ArrayBuffer>evt.data)
        let command = buf.readInt16BE(0)
        let len = buf.readInt32BE(2)
        console.info(`<<< received a message; command: ${command}, len: ${len}`)

        if (command == 101) {
          console.info('<<< received a pong message')
        }
      } catch (err) {
        console.error(evt.data, err)
      }
    }

    conn.onerror = err => {
      console.error('websocket error: ', err)
      this.errorHandler(err)
    }

    conn.onclose = (e: ICloseEvent) => {
      console.debug('event[onclose] fired')

      if (this.state == State.CLOSING) {
        this.onclose('logout')
        return
      }
    }

    this.conn = conn
    this.state = State.CONNECTED

    this.heartbeatLoop()
    this.readDeadlineLoop()

    return { status }
  }

  private async errorHandler(error: Error) {
    // 被踢或者主动logout
    if (this.state == State.CLOSED || this.state == State.CLOSING) {
      return
    }

    this.state = State.RECONNECTING

    console.debug(error)

    for (let index = 0; index < 10; index++) {
      try {
        console.info(`reconnecting... ${index + 1}`)

        let { status } = await this.login()

        if (status == 'Success') {
          return
        }
      } catch (error) {
        console.warn(error)
      }

      await sleep(5)
    }

    this.onclose('reconnect timeout')
  }

  private onclose(reason: string) {
    console.info('connection closed, due to: ', reason)

    this.state = State.CLOSED
  }

  logout() {
    if (this.state == State.CLOSING) {
      return
    }

    this.state = State.CLOSING
    if (!this.conn) {
      return
    }

    console.info('closing connection')

    this.conn.close()
  }

  private heartbeatLoop() {
    console.debug('heartbeatLoop started')
    let loop = () => {
      if (this.state != State.CONNECTED) {
        console.log('heartbeatLoop exit')
        return
      }

      console.log(`>>> send a ping message; state is ${this.state}`)

      this.send(Ping)

      setTimeout(loop, heartbeatInterval * 1000)
    }

    setTimeout(loop, heartbeatInterval * 1000)
  }

  private readDeadlineLoop() {
    console.debug('readDeadlineLoop started')
    let loop = () => {
        if(this.state != State.CONNECTED) {
            console.log('readDeadlineLoop exit')
            return
        }

        if(Date.now() - this.lastRead > 3 * heartbeatInterval * 1000) {
            this.errorHandler(new Error('read timeout'))
        }

        setTimeout(loop, 1000)
    }

    setTimeout(loop, 1000)
  }

  private send(data: Buffer | Uint8Array): boolean {
    try {
      if (this.conn == null) {
        return false
      }

      this.conn.send(data)
    } catch (error) {
      this.errorHandler(new Error('read timeout'))
      return false
    }

    return true
  }
}
