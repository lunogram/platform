import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { ProjectContext } from "@/contexts"
import { useContext, useState } from "react"

import { Button } from "@/components/ui/button"
import { ArrowRight, Mail, MessageSquareDot, PlusIcon, Smartphone, Webhook } from "lucide-react"
import type { ChannelType } from "@/types"

import {
    Item,
    ItemActions,
    ItemContent,
    ItemDescription,
    ItemGroup,
    ItemMedia,
    ItemTitle,
} from "@/components/ui/item"

import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"
import api from "@/api"

interface Channel {
    key: ChannelType;
    color: string;
    icon: JSX.Element;
    title: string;
    description: string;
}

interface CreateCampaignProps {
    open?: boolean;
}

export function CreateCampaign({ open = false }: CreateCampaignProps) {
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()
    const [isOpen, setIsOpen] = useState(open);

    async function create(channel: ChannelType) {
        const campaign = await api.campaigns.create(project.id, {
            name: generateProjectName(),
            channel: channel,
        })
        await navigate(`/projects/${project.id}/campaigns/${campaign.id}`)
    }

    const channels: Array<Channel> = [
        {
            key: 'email',
            color: 'bg-green-50 text-green-600',
            icon: <Mail strokeWidth={2} />,
            title: t('channels.email.title'),
            description: t('channels.email.description'),
        },
        {
            key: 'text',
            color: 'bg-blue-50 text-blue-600',
            icon: <Smartphone strokeWidth={2} />,
            title: t('channels.sms.title'),
            description: t('channels.sms.description'),
        },
        {
            key: 'push',
            color: 'bg-purple-50 text-purple-600',
            icon: <MessageSquareDot strokeWidth={2} />,
            title: t('channels.push.title'),
            description: t('channels.push.description'),
        },
        {
            key: 'webhook',
            color: 'bg-yellow-50 text-yellow-600',
            icon: <Webhook strokeWidth={2} />,
            title: t('channels.webhook.title'),
            description: t('channels.webhook.description'),
        },
    ]

    return (
        <Dialog open={isOpen} onOpenChange={() => setIsOpen(!isOpen)}>
            <DialogTrigger>
                <Button size="lg"><PlusIcon /> {t('campaign.create.action')}</Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t('campaign.create.title')}</DialogTitle>
                    <DialogDescription>
                        {t('campaign.create.description')}
                    </DialogDescription>
                </DialogHeader>

                <ItemGroup className="gap-2">
                    {channels.map((channel) => (
                        <Item key={channel.key} variant="outline" className="items-center" asChild>
                            <a className="no-underline cursor-pointer" onClick={() => create(channel.key)}>
                                <ItemMedia variant="icon" className={channel.color}>
                                    {channel.icon}
                                </ItemMedia>
                                <ItemContent>
                                    <ItemTitle>{channel.title}</ItemTitle>
                                    <ItemDescription>{channel.description}</ItemDescription>
                                </ItemContent>
                                <ItemActions>
                                    <ArrowRight strokeWidth={1} />
                                </ItemActions>
                            </a>
                        </Item>
                    ))}
                </ItemGroup>

            </DialogContent>
        </Dialog>
    )
}

const adjectives = [
    'admiring', 'adoring', 'affectionate', 'agitated', 'amazing',
    'angry', 'awesome', 'beautiful', 'blissful', 'bold',
    'boring', 'brave', 'busy', 'charming', 'clever',
    'cool', 'compassionate', 'competent', 'condescending', 'confident',
    'cranky', 'crazy', 'dazzling', 'determined', 'distracted',
    'dreamy', 'eager', 'ecstatic', 'elastic', 'elated',
    'elegant', 'eloquent', 'epic', 'exciting', 'fervent',
    'festive', 'flamboyant', 'focused', 'friendly', 'frosty',
    'funny', 'gallant', 'gifted', 'goofy', 'gracious',
    'happy', 'hardcore', 'heuristic', 'hopeful', 'hungry',
    'infallible', 'inspiring', 'intelligent', 'interesting', 'jolly',
    'jovial', 'keen', 'kind', 'laughing', 'loving',
    'lucid', 'magical', 'mystifying', 'modest', 'musing',
    'naughty', 'nervous', 'nice', 'nifty', 'nostalgic',
    'objective', 'optimistic', 'peaceful', 'pedantic', 'pensive',
    'practical', 'priceless', 'quirky', 'quizzical', 'recursing',
    'relaxed', 'reverent', 'romantic', 'sad', 'serene',
    'sharp', 'silly', 'sleepy', 'stoic', 'strange',
    'stupefied', 'suspicious', 'sweet', 'tender', 'thirsty',
    'trusting', 'unruffled', 'upbeat', 'vibrant', 'vigilant',
    'vigorous', 'wizardly', 'wonderful', 'xenodochial', 'youthful',
    'zealous', 'zen'
];

const names = [
    'albattani', 'allen', 'almeida', 'antonelli', 'agnesi',
    'archimedes', 'ardinghelli', 'aryabhata', 'austin', 'babbage',
    'banach', 'banzai', 'bardeen', 'bartik', 'bassi',
    'beaver', 'bell', 'benz', 'bhabha', 'bhaskara',
    'black', 'blackburn', 'blackwell', 'bohr', 'booth',
    'borg', 'bose', 'bouman', 'boyd', 'brahmagupta',
    'brattain', 'brown', 'buck', 'burnell', 'cannon',
    'carson', 'cartwright', 'carver', 'cerf', 'chandrasekhar',
    'chaplygin', 'chatelet', 'chatterjee', 'chebyshev', 'cohen',
    'chaum', 'clarke', 'colden', 'cori', 'cray',
    'curran', 'curie', 'darwin', 'davinci', 'dewdney',
    'dhawan', 'diffie', 'dijkstra', 'dirac', 'driscoll',
    'dubinsky', 'easley', 'edison', 'einstein', 'elbakyan',
    'elgamal', 'elion', 'ellis', 'engelbart', 'euclid',
    'euler', 'faraday', 'feistel', 'fermat', 'fermi',
    'feynman', 'franklin', 'gagarin', 'galileo', 'galois',
    'ganguly', 'gates', 'gauss', 'germain', 'goldberg',
    'goldstine', 'goldwasser', 'golick', 'goodall', 'gould',
    'greider', 'grothendieck', 'haibt', 'hamilton', 'haslett',
    'hawking', 'hellman', 'heisenberg', 'hermann', 'herschel',
    'hertz', 'heyrovsky', 'hodgkin', 'hofstadter', 'hoover',
    'hopper', 'hugle', 'hypatia', 'ishizaka', 'jackson',
    'jang', 'jemison', 'jennings', 'jepsen', 'johnson',
    'joliot', 'jones', 'kalam', 'kapitsa', 'kare',
    'keldysh', 'keller', 'kepler', 'khayyam', 'khorana',
    'kilby', 'kirch', 'knuth', 'kowalevski', 'lalande',
    'lamarr', 'lamport', 'leakey', 'leavitt', 'lederberg',
    'lehmann', 'lewin', 'lichterman', 'liskov', 'lovelace',
    'lumiere', 'mahavira', 'margulis', 'matsumoto', 'maxwell',
    'mayer', 'mccarthy', 'mcclintock', 'mclaren', 'mclean',
    'mcnulty', 'mendel', 'mendeleev', 'meitner', 'meninsky',
    'merkle', 'mestorf', 'mirzakhani', 'moore', 'morse',
    'murdock', 'moser', 'napier', 'nash', 'neumann',
    'newton', 'nightingale', 'nobel', 'noether', 'northcutt',
    'noyce', 'panini', 'pare', 'pascal', 'pasteur',
    'payne', 'perlman', 'pike', 'poincare', 'poitras',
    'proskuriakova', 'ptolemy', 'raman', 'ramanujan', 'ride',
    'montalcini', 'ritchie', 'rhodes', 'robinson', 'roentgen',
    'rosalind', 'rubin', 'saha', 'sammet', 'sanderson',
    'satoshi', 'shamir', 'shannon', 'shaw', 'shirley',
    'shockley', 'shtern', 'sinoussi', 'snyder', 'solomon',
    'spence', 'stonebraker', 'sutherland', 'swanson', 'swartz',
    'swirles', 'taussig', 'tereshkova', 'tesla', 'tharp',
    'thompson', 'torvalds', 'tu', 'turing', 'varahamihira',
    'vaughan', 'visvesvaraya', 'volhard', 'villani', 'wescoff',
    'wilbur', 'wiles', 'williams', 'williamson', 'wilson',
    'wing', 'wozniak', 'wright', 'wu', 'yalow',
    'yonath', 'zhukovsky'
];

function generateProjectName() {
    const adjective = adjectives[Math.floor(Math.random() * adjectives.length)];
    const name = names[Math.floor(Math.random() * names.length)];
    return `${adjective} ${name}`;
}
