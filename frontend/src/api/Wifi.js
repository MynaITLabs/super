import API from './index'

export class APIWifi extends API {
  constructor() {
    super('/hostapd')
  }

  allStations = () => this.get('/all_stations')
  status = () => this.get('/status')
}

export const wifiAPI = new APIWifi()