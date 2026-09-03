import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import App from './App.vue'

describe('App', () => {
  it('renders the admin workspace heading', () => {
    const wrapper = mount(App)

    expect(wrapper.get('main').text()).toContain('Diary Blog Admin')
    expect(wrapper.get('h1').text()).toBe('Diary Blog Admin')
  })
})
