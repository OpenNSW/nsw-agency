describe('test environment', () => {
  it('has jsdom document available', () => {
    expect(document).toBeDefined()
  })

  it('has jest-dom matchers extended', () => {
    const el = document.createElement('div')
    document.body.appendChild(el)
    expect(el).toBeInTheDocument()
    document.body.removeChild(el)
  })
})
