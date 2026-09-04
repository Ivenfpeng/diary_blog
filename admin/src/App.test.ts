import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import App from './App.vue'

describe('App', () => {
  it('renders the active route outlet', () => {
    const wrapper = mount(App, {
      global: {
        stubs: { RouterView: { template: '<main>Active route</main>' } },
      },
    })

    expect(wrapper.get('main').text()).toBe('Active route')
  })
})
