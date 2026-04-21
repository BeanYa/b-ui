export const DRAWER_COLLAPSED_WIDTH = 92
export const DRAWER_EXPANDED_WIDTH = 308
export const DRAWER_TRANSITION_MS = 200

export type DrawerPhase = 'collapsed' | 'collapsing' | 'expanded' | 'expanding'

type DrawerStateInput = {
  collapsed: boolean
  isMobile: boolean
}

type DrawerVisualStateInput = DrawerStateInput & {
  phase: DrawerPhase
}

type DrawerPhaseTransitionInput = DrawerStateInput & {
  currentPhase: DrawerPhase
  layoutChanged: boolean
}

export const getSettledDrawerPhase = (collapsed: boolean): DrawerPhase => collapsed ? 'collapsed' : 'expanded'

export const getNextDrawerPhase = ({ collapsed, isMobile }: DrawerStateInput): DrawerPhase => {
  if (isMobile) return 'expanded'
  return collapsed ? 'collapsing' : 'expanding'
}

export const planDrawerPhaseTransition = ({
  collapsed,
  currentPhase,
  isMobile,
  layoutChanged,
}: DrawerPhaseTransitionInput) => {
  const settledPhase = isMobile ? 'expanded' : getSettledDrawerPhase(collapsed)

  if (layoutChanged || currentPhase === settledPhase) {
    return {
      nextPhase: settledPhase,
      settleAfterMs: null,
    }
  }

  return {
    nextPhase: getNextDrawerPhase({ collapsed, isMobile }),
    settleAfterMs: DRAWER_TRANSITION_MS,
  }
}

export const getDrawerVisualState = ({ collapsed, isMobile, phase }: DrawerVisualStateInput) => {
  if (isMobile) {
    return {
      contentHidden: false,
      isRail: false,
      itemRail: false,
      width: DRAWER_EXPANDED_WIDTH,
    }
  }

  return {
    contentHidden: phase === 'collapsed' || phase === 'expanding',
    isRail: phase === 'collapsed',
    itemRail: phase === 'collapsed',
    width: collapsed ? DRAWER_COLLAPSED_WIDTH : DRAWER_EXPANDED_WIDTH,
  }
}
