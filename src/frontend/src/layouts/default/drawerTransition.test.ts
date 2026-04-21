import { describe, expect, it } from 'vitest'

import {
  DRAWER_TRANSITION_MS,
  getDrawerVisualState,
  getNextDrawerPhase,
  getSettledDrawerPhase,
  planDrawerPhaseTransition,
} from './drawerTransition'

describe('drawerTransition', () => {
  it('keeps content visible while desktop collapse is in progress', () => {
    expect(getNextDrawerPhase({ collapsed: true, isMobile: false })).toBe('collapsing')
    expect(getDrawerVisualState({ collapsed: true, isMobile: false, phase: 'collapsing' })).toEqual({
      contentHidden: false,
      isRail: false,
      itemRail: false,
      width: 92,
    })
  })

  it('keeps content hidden while desktop expand is in progress until it settles', () => {
    expect(getNextDrawerPhase({ collapsed: false, isMobile: false })).toBe('expanding')
    expect(getDrawerVisualState({ collapsed: false, isMobile: false, phase: 'expanding' })).toEqual({
      contentHidden: true,
      isRail: false,
      itemRail: false,
      width: 308,
    })
  })

  it('returns to true rail mode only after the collapsed phase settles', () => {
    expect(getSettledDrawerPhase(true)).toBe('collapsed')
    expect(getDrawerVisualState({ collapsed: true, isMobile: false, phase: 'collapsed' })).toEqual({
      contentHidden: true,
      isRail: true,
      itemRail: true,
      width: 92,
    })
  })

  it('keeps mobile drawer fully expanded regardless of the persisted desktop collapsed flag', () => {
    expect(getNextDrawerPhase({ collapsed: true, isMobile: true })).toBe('expanded')
    expect(getDrawerVisualState({ collapsed: true, isMobile: true, phase: 'expanded' })).toEqual({
      contentHidden: false,
      isRail: false,
      itemRail: false,
      width: 308,
    })
  })

  it('restarts the mirrored expand phase when a collapse is interrupted', () => {
    expect(planDrawerPhaseTransition({
      collapsed: false,
      currentPhase: 'collapsing',
      isMobile: false,
      layoutChanged: false,
    })).toEqual({
      nextPhase: 'expanding',
      settleAfterMs: DRAWER_TRANSITION_MS,
    })
  })

  it('snaps directly to the settled phase on breakpoint changes', () => {
    expect(planDrawerPhaseTransition({
      collapsed: true,
      currentPhase: 'expanded',
      isMobile: false,
      layoutChanged: true,
    })).toEqual({
      nextPhase: 'collapsed',
      settleAfterMs: null,
    })

    expect(planDrawerPhaseTransition({
      collapsed: true,
      currentPhase: 'collapsing',
      isMobile: true,
      layoutChanged: true,
    })).toEqual({
      nextPhase: 'expanded',
      settleAfterMs: null,
    })
  })
})
