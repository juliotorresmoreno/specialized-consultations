import React, { useEffect, useState } from 'react'
import { useParams } from 'react-router'
import Loading from '../components/Loading'
import config from '../config'
import { IChat } from '../models/chat'
import ErrorPage from '../pages/ErrorPage'
import { useAppSelector } from '../store/hooks'
import { HTTPError } from '../types/http'

type ResultProps = {
  [x: string | number | symbol]: any
} & React.PropsWithChildren

const url = config.baseUrl + '/chats/'

const withChat = function <T = any>(
  WrappedComponent: React.ComponentType<any>
) {
  const result: React.FC<T & ResultProps> = (props) => {
    const session = useAppSelector((state) => state.auth.session)
    const [isloading, setIsloading] = useState<boolean>(false)
    const [chat, setChat] = useState<IChat | null>(null)
    const [error, setError] = useState<HTTPError | null>(null)
    const id = useParams<{ id: string }>().id as string

    useEffect(() => {
      setIsloading(true)
      setError(null)

      fetch(url + id, {
        headers: {
          'X-API-Key': session?.token ?? '',
        },
      })
        .then(async (response) => {
          const content = await response.json()
          if (!response.ok) {
            setError(content)
            return
          }
          setChat(content)
        })
        .catch((err: Error) => setError({ message: err.message }))
        .finally(() => setIsloading(false))
    }, [session, id])

    if (error) return <ErrorPage error={error} />
    if (!chat || isloading) return <Loading />
    return <WrappedComponent chat={chat} {...props} />
  }
  return result
}

export default withChat
