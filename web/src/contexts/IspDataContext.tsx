import type { ReactNode } from "react"
import { createContext, useCallback, useContext, useState } from "react"

export type IspDataMap = Record<string, string | null | "loading">

interface IspDataContextType {
  ispData: IspDataMap
  setIspData: (data: IspDataMap | ((prev: IspDataMap) => IspDataMap)) => void
  clearIspData: () => void
}

const IspDataContext = createContext<IspDataContextType | undefined>(undefined)

export function IspDataProvider({ children }: { children: ReactNode }) {
  const [ispData, setIspDataState] = useState<IspDataMap>({})

  const setIspData = useCallback((data: IspDataMap | ((prev: IspDataMap) => IspDataMap)) => {
    setIspDataState(data)
  }, [])

  const clearIspData = useCallback(() => {
    setIspDataState({})
  }, [])

  return (
    <IspDataContext.Provider value={{ ispData, setIspData, clearIspData }}>
      {children}
    </IspDataContext.Provider>
  )
}

export function useIspData() {
  const context = useContext(IspDataContext)
  if (context === undefined) {
    throw new Error("useIspData must be used within an IspDataProvider")
  }
  return context
}
